package repository

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// ErrDuplicateURL возвращается при попытке сохранить originalURL, который уже
// есть в хранилище.
var ErrDuplicateURL = errors.New("duplicate URL")

const pgUniqueViolationCode = "23505"

// DBStorage хранит записи в PostgreSQL и применяет SQL-миграции при инициализации.
type DBStorage struct {
	db         *sql.DB
	migrations string
}

// NewDBStorage создаёт хранилище PostgreSQL по DSN и проверяет соединение.
// Возвращает ошибку, если драйвер недоступен или база не отвечает.
func NewDBStorage(dsn string) (*DBStorage, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к БД: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ошибка проверки подключения к БД: %w", err)
	}

	return &DBStorage{
		db:         db,
		migrations: "migrations",
	}, nil
}

// Init применяет миграции из директории migrations и подготавливает схему.
// Возвращает ошибку, если директория не найдена или SQL выполнился с ошибкой.
func (s *DBStorage) Init() error {
	if err := s.runMigrations(); err != nil {
		return fmt.Errorf("ошибка миграций: %w", err)
	}
	return nil
}

//go:embed migrations/*.sql
var embedMigrations embed.FS

// runMigrations применяет миграции goose из встроенных SQL-файлов.
func (s *DBStorage) runMigrations() error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	if err := goose.Up(s.db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// Insert добавляет пару originalURL→shortURL, привязывая её к userID.
// Возвращает ErrDuplicateURL при нарушении уникальности original_url.
func (s *DBStorage) Insert(userID string, originalURL string, shortURL string) error {
	_, err := s.db.Exec(
		"INSERT INTO urls (short_url, original_url, user_id) VALUES ($1, $2, $3)",
		shortURL, originalURL, userID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
			return ErrDuplicateURL
		}
		return fmt.Errorf("ошибка вставки в БД: %w", err)
	}
	return nil
}

// BatchInsert добавляет несколько записей в рамках одной транзакции
// и возвращает результаты в том же порядке с фактическим short_url:
// для дубликатов — существующий, для новых — вставленный.
func (s *DBStorage) BatchInsert(userID string, records []URLRecord) ([]URLRecord, error) {
	if len(records) == 0 {
		return []URLRecord{}, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	// Откат транзакции игнорируем: если Commit прошёл успешно,
	// Rollback вернёт ErrTxDone — это ожидаемое поведение.
	defer func() { _ = tx.Rollback() }()

	// Собираем original_url для последующего SELECT
	originalURLs := make([]string, len(records))
	for i, record := range records {
		originalURLs[i] = record.OriginalURL
	}

	// Вставляем новые записи, дубликаты игнорируем
	stmt, err := tx.Prepare(
		"INSERT INTO urls (short_url, original_url, user_id) VALUES ($1, $2, $3) ON CONFLICT (original_url) DO NOTHING",
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка подготовки запроса: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, record := range records {
		if _, err = stmt.Exec(record.ShortURL, record.OriginalURL, userID); err != nil {
			return nil, fmt.Errorf("ошибка вставки записи %s: %w", record.ShortURL, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("ошибка фиксации транзакции: %w", err)
	}

	// Возвращаем все short_url — и вставленные, и существующие
	results, err := s.selectByOriginalURLs(originalURLs)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения записей: %w", err)
	}

	resultMap := make(map[string]URLRecord, len(results))
	for _, r := range results {
		resultMap[r.OriginalURL] = r
	}

	orderedResults := make([]URLRecord, len(records))
	for i, record := range records {
		r, ok := resultMap[record.OriginalURL]
		if !ok {
			return nil, fmt.Errorf("запись не найдена для original_url: %s", record.OriginalURL)
		}
		orderedResults[i] = r
	}

	return orderedResults, nil
}

// selectByOriginalURLs возвращает записи по списку original_url
func (s *DBStorage) selectByOriginalURLs(originalURLs []string) ([]URLRecord, error) {
	rows, err := s.db.Query(
		"SELECT short_url, original_url FROM urls WHERE original_url = ANY($1)",
		originalURLs,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска записей: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []URLRecord
	for rows.Next() {
		var record URLRecord
		if err := rows.Scan(&record.ShortURL, &record.OriginalURL); err != nil {
			return nil, fmt.Errorf("ошибка сканирования записи: %w", err)
		}
		results = append(results, record)
	}

	return results, rows.Err()
}

// Select возвращает запись по короткому адресу и false, если записи нет
// либо запрос к базе завершился ошибкой.
func (s *DBStorage) Select(shortURL string) (URLRecord, bool) {
	var record URLRecord
	err := s.db.QueryRow(
		"SELECT short_url, original_url, is_deleted FROM urls WHERE short_url = $1",
		shortURL,
	).Scan(&record.ShortURL, &record.OriginalURL, &record.DeletedFlag)

	if errors.Is(err, sql.ErrNoRows) {
		return record, false
	}
	if err != nil {
		return record, false
	}

	return record, true
}

// SelectByUser возвращает все неудалённые URL, сокращённые указанным пользователем.
// При ошибке запроса возвращает nil.
func (s *DBStorage) SelectByUser(userID string) []URLRecord {
	rows, err := s.db.Query(
		"SELECT short_url, original_url FROM urls WHERE user_id = $1 AND is_deleted = FALSE",
		userID,
	)
	if err != nil {
		slog.Error("ошибка запроса SelectByUser", "error", err, "user_id", userID)
		return nil
	}
	defer func() { _ = rows.Close() }()

	var results []URLRecord
	for rows.Next() {
		var record URLRecord
		if err := rows.Scan(&record.ShortURL, &record.OriginalURL); err != nil {
			slog.Error("ошибка сканирования", "error", err)
			return nil
		}
		results = append(results, record)
	}

	if err := rows.Err(); err != nil {
		slog.Error("ошибка итерации", "error", err)
		return nil
	}

	return results
}

// DeleteByUser помечает URL удалёнными множественным обновлением (batch update).
// Условие по user_id гарантирует, что удалить URL может только его создатель.
func (s *DBStorage) DeleteByUser(userID string, shortURLs []string) error {
	if len(shortURLs) == 0 {
		return nil
	}

	if _, err := s.db.Exec(
		"UPDATE urls SET is_deleted = TRUE WHERE user_id = $1 AND short_url = ANY($2)",
		userID, shortURLs,
	); err != nil {
		return fmt.Errorf("ошибка удаления записей: %w", err)
	}

	return nil
}

// Close закрывает пул соединений с базой данных; повторный вызов безопасен.
func (s *DBStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// DB возвращает нижележащий *sql.DB, например для проверки доступности БД.
func (s *DBStorage) DB() *sql.DB {
	return s.db
}
