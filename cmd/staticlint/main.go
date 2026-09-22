// Command staticlint — статический анализатор для проекта shortener.
//
// # Запуск
//
// Соберите бинарник и запустите его, передав пути к анализируемым пакетам:
//
//	go build -o staticlint ./cmd/staticlint
//	./staticlint ./...
//
// Или запустите без сборки:
//
//	go run ./cmd/staticlint ./...
//
// # Подключённые анализаторы
//
// ## Стандартные анализаторы golang.org/x/tools/go/analysis/passes
//
//   - printf: проверяет соответствие форматных спецификаторов и аргументов
//   - shadow: ищет затенённые переменные
//   - structtag: проверяет теги структур
//   - unreachable: ищет недостижимый код
//   - loopclosure: ищет замыкания на переменные цикла
//   - copylock: ищет копирование мьютексов
//   - lostcancel: ищет забытые вызовы cancel()
//   - errorsas: проверяет корректность errors.As
//   - httpresponse: ищет ошибки при работе с HTTP-ответами
//   - ifaceassert: проверяет невозможные type assertion
//   - shift: проверяет сдвиги за пределы разрядности
//   - slog: проверяет вызовы log/slog
//   - stdmethods: проверяет сигнатуры стандартных методов
//   - stringintconv: ищет преобразования string(int)
//   - unmarshal: проверяет корректность json.Unmarshal
//   - unsafeptr: проверяет корректность unsafe.Pointer
//   - unusedresult: ищет неиспользуемые результаты функций
//   - waitgroup: проверяет корректность sync.WaitGroup
//
// ## Анализаторы staticcheck.io
//
//   - Все анализаторы класса SA (SA1xxx–SA9xxx): ошибки и подозрительные конструкции
//   - S1002: упрощение булевых выражений
//   - ST1005: стиль сообщений об ошибках
//   - QF1003: рефакторинг if/else в switch
//
// ## Публичные анализаторы
//
//   - errcheck (github.com/kisielk/errcheck): проверяет необработанные ошибки
//   - gocognit (github.com/uudashr/gocognit): оценивает когнитивную сложность
//
// ## Собственный анализатор
//
//   - osexit: запрещает прямой вызов os.Exit в функции main пакета main
package main

import (
	"log"
	"strings"

	"github.com/b602op/shortener/cmd/staticlint/osexit"
	"github.com/kisielk/errcheck/errcheck"
	"github.com/uudashr/gocognit"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/slog"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/waitgroup"
	"honnef.co/go/tools/quickfix"
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
)

func main() {
	var analyzers []*analysis.Analyzer

	// 1. Стандартные анализаторы из golang.org/x/tools/go/analysis/passes
	analyzers = append(analyzers,
		printf.Analyzer,
		shadow.Analyzer,
		structtag.Analyzer,
		unreachable.Analyzer,
		loopclosure.Analyzer,
		copylock.Analyzer,
		lostcancel.Analyzer,
		errorsas.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		shift.Analyzer,
		slog.Analyzer,
		stdmethods.Analyzer,
		stringintconv.Analyzer,
		unmarshal.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
		waitgroup.Analyzer,
	)

	// 2. Анализаторы staticcheck.io: все SA + выборочно S, ST, QF
	for _, v := range staticcheck.Analyzers {
		if strings.HasPrefix(v.Analyzer.Name, "SA") {
			analyzers = append(analyzers, v.Analyzer)
		}
	}
	// Один анализатор из класса S (Simple)
	for _, v := range simple.Analyzers {
		if v.Analyzer.Name == "S1002" {
			analyzers = append(analyzers, v.Analyzer)
		}
	}
	// Один анализатор из класса ST (Style)
	for _, v := range stylecheck.Analyzers {
		if v.Analyzer.Name == "ST1005" {
			analyzers = append(analyzers, v.Analyzer)
		}
	}
	// Один анализатор из класса QF (Quickfix)
	for _, v := range quickfix.Analyzers {
		if v.Analyzer.Name == "QF1003" {
			analyzers = append(analyzers, v.Analyzer)
		}
	}

	// 3. Публичные анализаторы на выбор (2+)
	analyzers = append(analyzers,
		errcheck.Analyzer,
		gocognit.Analyzer,
	)

	// Настраиваем порог когнитивной сложности для gocognit.
	// Функции сложнее 30 считаются слишком запутанными и требуют рефакторинга.
	if err := gocognit.Analyzer.Flags.Set("over", "30"); err != nil {
		log.Printf("Не удалось установить порог gocognit: %v", err)
	}

	// 4. Собственный анализатор
	analyzers = append(analyzers, osexit.Analyzer)

	// Запускаем multichecker со всеми анализаторами
	multichecker.Main(analyzers...)
}
