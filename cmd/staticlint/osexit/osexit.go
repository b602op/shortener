// Package osexit предоставляет анализатор, запрещающий прямой вызов
// os.Exit в функции main пакета main.
//
// Использование os.Exit в main обходит механизмы graceful shutdown
// и мешает корректному освобождению ресурсов (defer-функции не выполняются).
// Вместо os.Exit следует возвращать код ошибки из run() и обрабатывать
// его в main через log.Fatal или os.Exit только в самом main.
package osexit

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer — анализатор, запрещающий os.Exit в main.
var Analyzer = &analysis.Analyzer{
	Name: "osexit",
	Doc:  "prohibits direct calls to os.Exit in the main function",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Проверяем только пакеты с именем main.
	// pass.Pkg может быть nil в редких случаях — защищаемся.
	if pass.Pkg == nil || pass.Pkg.Name() != "main" {
		return nil, nil
	}

	for _, file := range pass.Files {
		// Пропускаем сгенерированные файлы (например, _testmain.go,
		// который компилятор создаёт для запуска тестов и который сам
		// содержит os.Exit(m.Run())).
		if isGenerated(file) {
			continue
		}

		// Ищем функцию main
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "main" {
				continue
			}

			// Обходим тело main, включая анонимные функции и замыкания
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				// Проверяем, что это вызов вида os.Exit
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				// Проверяем, что слева — идентификатор os
				ident, ok := sel.X.(*ast.Ident)
				if !ok || ident.Name != "os" {
					return true
				}

				// Проверяем, что метод — Exit
				if sel.Sel.Name != "Exit" {
					return true
				}

				pass.Reportf(call.Pos(), "direct call to os.Exit in main is prohibited; use run() + return error instead")
				return true
			})
		}
	}

	return nil, nil
}

// isGenerated сообщает, является ли файл сгенерированным автоматически.
// Сгенерированные файлы помечаются комментарием вида
// "// Code generated ... DO NOT EDIT." в начале файла.
func isGenerated(file *ast.File) bool {
	for _, cg := range file.Comments {
		// Смотрим только комментарии перед объявлением пакета.
		if cg.Pos() > file.Package {
			break
		}
		text := cg.Text()
		if strings.Contains(text, "Code generated") && strings.Contains(text, "DO NOT EDIT") {
			return true
		}
	}
	return false
}
