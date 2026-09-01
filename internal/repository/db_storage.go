package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// ErrDuplicateURL возвращается при попытке вставить дублирующую запись
var ErrDuplicateURL = errors.New("duplicate URL")

const pgUniqueViolationCode = "23505"

// DBStorage — хранение в PostgreSQL с миграциями
type DBStorage struct {
	db         *sql.DB
	migrations string
}

// NewDBStorage создаёт новое хранилище PostgreSQL
func NewDBStorage(dsn string) (*DBStorage, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к БД: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ошибка проверки подключения к БД: %w", err)
	}

	return &DBStorage{
		db:         db,
		migrations: "migrations",
	}, nil
}

// Init выполняет миграции и инициализирует хранилище
func (s *DBStorage) Init() error {
	if err := s.runMigrations(); err != nil {
		return fmt.Errorf("ошибка миграций: %w", err)
	}
	return nil
}

// runMigrations применяет SQL-миграции из директории
func (s *DBStorage) runMigrations() error {
	migrationsDir := s.findMigrationsDir()
	if migrationsDir == "" {
		return fmt.Errorf("директория миграций не найдена")
	}

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("ошибка чтения директории миграций: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		path := filepath.Join(migrationsDir, file.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("ошибка чтения миграции %s: %w", file.Name(), err)
		}

		if err := s.executeSQL(string(content)); err != nil {
			return fmt.Errorf("ошибка выполнения миграции %s: %w", file.Name(), err)
		}
	}

	return nil
}

// findMigrationsDir ищет директорию миграций относительно рабочей директории
func (s *DBStorage) findMigrationsDir() string {
	// Путь относительно рабочей директории
	if _, err := os.Stat(s.migrations); err == nil {
		return s.migrations
	}

	// Путь относительно директории пакета
	candidates := []string{
		"migrations",
		"../migrations",
		"../../migrations",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// executeSQL выполняет SQL-запрос, разделяя по блокам Up/Down
func (s *DBStorage) executeSQL(sqlContent string) error {
	upSQL := extractUpBlock(sqlContent)
	if upSQL == "" {
		return nil
	}

	statements := splitStatements(upSQL)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("ошибка выполнения SQL: %w", err)
		}
	}

	return nil
}

// extractUpBlock извлекает блок SQL между +goose Up и +goose StatementBegin/End
func extractUpBlock(content string) string {
	upStart := strings.Index(content, "-- +goose Up")
	if upStart == -1 {
		return content
	}

	downStart := strings.Index(content, "-- +goose Down")
	if downStart == -1 {
		downStart = len(content)
	}

	block := content[upStart:downStart]

	// Убираем комментарии +goose
	lines := strings.Split(block, "\n")
	var sqlLines []string
	skipGoose := true
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- +goose") {
			continue
		}
		if skipGoose && trimmed == "" {
			continue
		}
		skipGoose = false
		sqlLines = append(sqlLines, line)
	}

	return strings.Join(sqlLines, "\n")
}

// splitStatements разделяет SQL на отдельные операторы
func splitStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	depth := 0

	lines := strings.Split(sql, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Пропускаем пустые строки и комментарии
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			if current.Len() > 0 {
				current.WriteRune('\n')
			}
			continue
		}

		for _, ch := range line {
			if ch == '(' {
				depth++
			} else if ch == ')' {
				depth--
			} else if ch == ';' && depth == 0 {
				stmt := strings.TrimSpace(current.String())
				if stmt != "" {
					statements = append(statements, stmt)
				}
				current.Reset()
				continue
			}
			current.WriteRune(ch)
		}
		current.WriteRune('\n')
	}

	// Добавляем последний оператор без точки с запятой
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// Insert добавляет запись в базу данных
func (s *DBStorage) Insert(originalURL string, shortURL string) error {
	_, err := s.db.Exec(
		"INSERT INTO urls (short_url, original_url) VALUES ($1, $2)",
		shortURL, originalURL,
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
// и возвращает результаты в том же порядке с actual short_url
// (для дубликатов — существующий, для новых — вставленный)
func (s *DBStorage) BatchInsert(records []URLRecord) ([]URLRecord, error) {
	if len(records) == 0 {
		return []URLRecord{}, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback()

	// Собираем original_url для последующего SELECT
	originalURLs := make([]string, len(records))
	for i, record := range records {
		originalURLs[i] = record.OriginalURL
	}

	// Вставляем новые записи, дубликаты игнорируем
	stmt, err := tx.Prepare(
		"INSERT INTO urls (short_url, original_url) VALUES ($1, $2) ON CONFLICT (original_url) DO NOTHING",
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка подготовки запроса: %w", err)
	}
	defer stmt.Close()

	for _, record := range records {
		_, err := stmt.Exec(record.ShortURL, record.OriginalURL)
		if err != nil {
			return nil, fmt.Errorf("ошибка вставки записи %s: %w", record.ShortURL, err)
		}
	}

	if err := tx.Commit(); err != nil {
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
	defer rows.Close()

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

// Select возвращает оригинальный URL по короткому
func (s *DBStorage) Select(shortURL string) (URLRecord, bool) {
	var record URLRecord
	err := s.db.QueryRow(
		"SELECT short_url, original_url FROM urls WHERE short_url = $1",
		shortURL,
	).Scan(&record.ShortURL, &record.OriginalURL)

	if err == sql.ErrNoRows {
		return record, false
	}
	if err != nil {
		return record, false
	}

	return record, true
}

// Close закрывает подключение к базе данных
func (s *DBStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// DB возвращает объект database/sql.DB
func (s *DBStorage) DB() *sql.DB {
	return s.db
}
