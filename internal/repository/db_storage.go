package repository

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

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
		return fmt.Errorf("ошибка вставки в БД: %w", err)
	}
	return nil
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
