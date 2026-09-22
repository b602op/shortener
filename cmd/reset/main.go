// Package main реализует утилиту кодогенерации reset.
//
// Утилита рекурсивно сканирует пакеты проекта, начиная с корневой директории,
// находит структуры с директивой // generate:reset и генерирует для каждой
// метод Reset(), возвращающий состояние объекта к нулевым значениям.
// Все методы одного пакета записываются в файл reset.gen.go этого же пакета.
//
// # Запуск
//
// Из корня проекта:
//
//	go run ./cmd/reset
//
// С явным указанием корневой директории:
//
//	go run ./cmd/reset -dir ./internal
//
// Через go generate (директива //go:generate в cmd/shortener/main.go):
//
//	go generate ./...
//
// # Правила генерации Reset()
//
//   - примитивы (числа, string, bool) — нулевые значения: 0, "", false;
//   - слайсы — обрезаются до нулевой длины без зануления элементов: s = s[:0];
//   - мапы — очищаются встроенной функцией clear;
//   - указатели — значение по указателю сбрасывается, если указатель не nil;
//   - вложенные структуры — вызывается их Reset(), а при его отсутствии
//     поля сбрасываются рекурсивно;
//   - интерфейсы — сбрасываются через type assertion на interface{ Reset() };
//   - каналы, функции, массивы и типы из внешних пакетов — не сбрасываются
//     (помечаются комментарием в сгенерированном коде).
//
// # Пример
//
// Директива над структурой:
//
//	// generate:reset
//	type Counter struct {
//	    N    int
//	    Tags []string
//	}
//
// порождает в reset.gen.go метод:
//
//	func (rs *Counter) Reset() {
//	    if rs == nil {
//	        return
//	    }
//	    rs.N = 0
//	    rs.Tags = rs.Tags[:0]
//	}
//
// Если в пакете нет структур с директивой, файл reset.gen.go не создаётся,
// а устаревший — удаляется.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// directive — маркерная строка директивы генерации.
const directive = "generate:reset"

// generatedFileName — имя файла с сгенерированными методами Reset.
const generatedFileName = "reset.gen.go"

// structInfo — структура с директивой // generate:reset.
type structInfo struct {
	name   string
	fields []fieldInfo
}

// fieldInfo — имя и тип поля сбрасываемой структуры.
type fieldInfo struct {
	name string
	typ  ast.Expr
}

// pkgInfo — сведения о пакете, необходимые для генерации.
type pkgInfo struct {
	name      string                     // имя пакета
	structs   map[string]*ast.StructType // локальные структуры
	baseTypes map[string]ast.Expr        // локальные именованные типы и их основа
	hasReset  map[string]bool            // типы с методом Reset (объявленным или сгенерированным)
	directed  []structInfo               // структуры с директивой
}

// newPkgInfo создаёт пустое описание пакета.
func newPkgInfo() *pkgInfo {
	return &pkgInfo{
		structs:   make(map[string]*ast.StructType),
		baseTypes: make(map[string]ast.Expr),
		hasReset:  make(map[string]bool),
	}
}

func main() {
	dir := flag.String("dir", ".", "корневая директория проекта для сканирования")
	flag.Parse()

	if err := run(*dir); err != nil {
		log.Printf("reset: %v", err)
	}
}

// run сканирует пакеты от root рекурсивно вниз и генерирует reset.gen.go
// в каждом пакете, где найдены структуры с директивой // generate:reset.
func run(root string) error {
	dirs, err := goDirs(root)
	if err != nil {
		return fmt.Errorf("ошибка обхода %s: %w", root, err)
	}

	for _, dir := range dirs {
		n, err := generateForDir(dir)
		if err != nil {
			return err
		}
		if n > 0 {
			log.Printf("reset: %s — сгенерировано методов: %d", dir, n)
		}
	}
	return nil
}

// goDirs обходит root рекурсивно и возвращает отсортированный список
// директорий, содержащих хотя бы один .go-файл.
func goDirs(root string) ([]string, error) {
	seen := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") {
			seen[filepath.Dir(path)] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	dirs := make([]string, 0, len(seen))
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs, nil
}

// skipDir сообщает, следует ли пропустить директорию при обходе:
// скрытые, vendor, testdata и node_modules не анализируются.
func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") ||
		name == "vendor" || name == "testdata" || name == "node_modules"
}

// generateForDir анализирует пакет в директории dir и при наличии структур
// с директивой // generate:reset создаёт (перезаписывает) файл reset.gen.go.
// Если директив в пакете нет, устаревший файл удаляется.
// Возвращает число сгенерированных методов.
func generateForDir(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("ошибка чтения директории %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return 0, nil
	}

	pkg := newPkgInfo()
	fset := token.NewFileSet()
	for _, name := range files {
		f, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if parseErr != nil {
			return 0, fmt.Errorf("ошибка парсинга %s: %w", name, parseErr)
		}
		collectFile(f, pkg)
	}

	out := filepath.Join(dir, generatedFileName)

	if len(pkg.directed) == 0 {
		// Директив нет: удаляем устаревший сгенерированный файл, если он есть.
		if _, err := os.Stat(out); err == nil {
			if err := os.Remove(out); err != nil {
				return 0, fmt.Errorf("ошибка удаления устаревшего %s: %w", generatedFileName, err)
			}
		}
		return 0, nil
	}

	code, renderErr := renderPackage(pkg)
	if renderErr != nil {
		return 0, fmt.Errorf("ошибка генерации кода для %s: %w", dir, renderErr)
	}
	if err := os.WriteFile(out, code, 0o644); err != nil {
		return 0, fmt.Errorf("ошибка записи %s: %w", out, err)
	}
	return len(pkg.directed), nil
}

// collectFile извлекает из файла сведения для генерации: локальные типы,
// структуры с директивой // generate:reset и типы с методом Reset.
func collectFile(f *ast.File, pkg *pkgInfo) {
	if pkg.name == "" {
		pkg.name = f.Name.Name
	}

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				pkg.structs[ts.Name.Name] = st
			} else {
				pkg.baseTypes[ts.Name.Name] = ts.Type
			}
			if hasDirective(ts.Doc) || hasDirective(gd.Doc) {
				pkg.directed = append(pkg.directed, newStructInfo(ts))
				pkg.hasReset[ts.Name.Name] = true
			}
		}
	}

	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil || fd.Name.Name != "Reset" {
			continue
		}
		if name := recvTypeName(fd.Recv.List[0].Type); name != "" {
			pkg.hasReset[name] = true
		}
	}
}

// hasDirective сообщает, содержит ли группа комментариев директиву
// // generate:reset. Директива обязана занимать отдельную строку комментария
// целиком: упоминание директивы в тексте godoc не считается.
func hasDirective(cg *ast.CommentGroup) bool {
	if cg == nil {
		return false
	}
	for _, c := range cg.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if text == directive {
			return true
		}
	}
	return false
}

// newStructInfo собирает имя и поля структуры из TypeSpec.
func newStructInfo(ts *ast.TypeSpec) structInfo {
	info := structInfo{name: ts.Name.Name}
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return info
	}
	for _, field := range st.Fields.List {
		for _, name := range field.Names {
			info.fields = append(info.fields, fieldInfo{name: name.Name, typ: field.Type})
		}
	}
	return info
}

// recvTypeName возвращает имя типа ресивера метода: для (s *T) и (s T) — T.
func recvTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

// renderPackage собирает код файла reset.gen.go для пакета и форматирует его.
func renderPackage(pkg *pkgInfo) ([]byte, error) {
	var b strings.Builder
	b.WriteString("// Code generated by cmd/reset. DO NOT EDIT.\n\n")
	b.WriteString("package " + pkg.name + "\n\n")
	for _, s := range pkg.directed {
		renderMethod(&b, s, pkg)
	}

	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("некорректный сгенерированный код: %w", err)
	}
	return formatted, nil
}

// renderMethod записывает в b метод Reset для структуры s.
func renderMethod(b *strings.Builder, s structInfo, pkg *pkgInfo) {
	fmt.Fprintf(b, "// Reset сбрасывает состояние %s к нулевым значениям.\n", s.name)
	fmt.Fprintf(b, "// Сгенерировано утилитой cmd/reset по директиве // %s.\n", directive)
	fmt.Fprintf(b, "func (rs *%s) Reset() {\n", s.name)
	b.WriteString("\tif rs == nil {\n\t\treturn\n\t}\n")

	visited := map[string]bool{s.name: true}
	for _, f := range s.fields {
		for _, stmt := range resetExprStmts("rs."+f.name, f.typ, pkg, visited) {
			b.WriteString("\t" + stmt + "\n")
		}
	}
	b.WriteString("}\n\n")
}

// resetExprStmts возвращает список операторов, сбрасывающих значение выражения
// prefix (например, rs.Field) в зависимости от его типа.
func resetExprStmts(prefix string, typ ast.Expr, pkg *pkgInfo, visited map[string]bool) []string {
	switch t := typ.(type) {
	case *ast.Ident:
		return resetIdentStmts(prefix, t.Name, pkg, visited)
	case *ast.StarExpr:
		inner := resetDerefStmts(prefix, t.X, pkg, visited)
		if len(inner) == 0 {
			return []string{"// " + prefix + ": указатель на этот тип не сбрасывается"}
		}
		stmts := []string{"if " + prefix + " != nil {"}
		for _, s := range inner {
			stmts = append(stmts, "\t"+s)
		}
		return append(stmts, "}")
	case *ast.ArrayType:
		if t.Len == nil {
			return []string{prefix + " = " + prefix + "[:0]"}
		}
		return []string{"// " + prefix + ": массив не сбрасывается"}
	case *ast.MapType:
		return []string{"clear(" + prefix + ")"}
	case *ast.InterfaceType:
		return []string{
			"if resetter, ok := " + prefix + ".(interface{ Reset() }); ok {",
			"\tresetter.Reset()",
			"}",
		}
	case *ast.ChanType:
		return []string{"// " + prefix + ": канал не сбрасывается"}
	case *ast.FuncType:
		return []string{"// " + prefix + ": функция не сбрасывается"}
	default:
		return []string{"// " + prefix + ": поле этого типа не сбрасывается"}
	}
}

// resetIdentStmts возвращает операторы сброса выражения prefix именованного типа name.
func resetIdentStmts(prefix, name string, pkg *pkgInfo, visited map[string]bool) []string {
	if zero, ok := zeroValue(name); ok {
		return []string{prefix + " = " + zero}
	}
	// Структура с методом Reset сбрасывается вызовом этого метода —
	// даже если тип рекурсивный (например, поле child *T внутри T).
	if pkg.hasReset[name] {
		return []string{prefix + ".Reset()"}
	}
	if visited[name] {
		return []string{"// " + prefix + ": рекурсивный тип " + name + " не сбрасывается"}
	}
	if base, ok := pkg.baseTypes[name]; ok {
		return resetExprStmts(prefix, base, pkg, cloneVisited(visited, name))
	}
	if st, ok := pkg.structs[name]; ok {
		return resetStructFieldsStmts(prefix, st, pkg, cloneVisited(visited, name))
	}
	return []string{"// " + prefix + ": тип " + name + " не сбрасывается"}
}

// resetDerefStmts возвращает операторы сброса значения по указателю prefix
// (то есть *prefix). Пустой срез означает, что тип не сбрасывается.
func resetDerefStmts(prefix string, typ ast.Expr, pkg *pkgInfo, visited map[string]bool) []string {
	switch t := typ.(type) {
	case *ast.Ident:
		if zero, ok := zeroValue(t.Name); ok {
			return []string{"*" + prefix + " = " + zero}
		}
		if pkg.hasReset[t.Name] {
			return []string{prefix + ".Reset()"}
		}
		if visited[t.Name] {
			return nil
		}
		if base, ok := pkg.baseTypes[t.Name]; ok {
			return resetDerefStmts(prefix, base, pkg, cloneVisited(visited, t.Name))
		}
		if st, ok := pkg.structs[t.Name]; ok {
			return resetStructFieldsStmts(prefix, st, pkg, cloneVisited(visited, t.Name))
		}
		return nil
	case *ast.ArrayType:
		if t.Len == nil {
			return []string{"*" + prefix + " = (*" + prefix + ")[:0]"}
		}
		return nil
	case *ast.MapType:
		return []string{"clear(*" + prefix + ")"}
	default:
		return nil
	}
}

// resetStructFieldsStmts возвращает операторы, сбрасывающие все поля
// структуры st, доступной через выражение prefix.
func resetStructFieldsStmts(prefix string, st *ast.StructType, pkg *pkgInfo, visited map[string]bool) []string {
	var stmts []string
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			stmts = append(stmts, "// "+prefix+": встроенное поле не сбрасывается")
			continue
		}
		for _, name := range field.Names {
			stmts = append(stmts, resetExprStmts(prefix+"."+name.Name, field.Type, pkg, visited)...)
		}
	}
	return stmts
}

// zeroValue возвращает литерал нулевого значения предобъявленного типа name
// и признак того, что тип сбрасывается присваиванием.
func zeroValue(name string) (string, bool) {
	switch name {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"uintptr", "byte", "rune",
		"float32", "float64",
		"complex64", "complex128":
		return "0", true
	case "string":
		return `""`, true
	case "bool":
		return "false", true
	case "any", "error":
		return "nil", true
	}
	return "", false
}

// cloneVisited копирует множество посещённых типов и добавляет name.
func cloneVisited(visited map[string]bool, name string) map[string]bool {
	v := make(map[string]bool, len(visited)+1)
	for k := range visited {
		v[k] = true
	}
	v[name] = true
	return v
}
