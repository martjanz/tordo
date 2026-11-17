package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var spanishChars = regexp.MustCompile(`[áéíóúñ¿¡ÁÉÍÓÚÑ]`)

// Common Spanish words without accents, to catch messages that the accent
// check misses.
var spanishWords = regexp.MustCompile(`(?i)\b(de|del|en|con|que|por|para|una?|se|es|sin|falta|faltan|hay|ya|desde|tras|para|los|las|el|la|volumen|rockola|escaneo|respuesta|pedido|miniatura|cabecera|corto|paquete|demasiado|formato|archivo|carpeta|red local|pista|desaparecio|ilegible|invalido|invalida|pudo|ejecutarse|reproducirse|necesita|salir|truncad[oa]s?)\b`)

// messageCall reports whether a call expression is one whose literal
// arguments are user-visible messages.
func messageCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	name := sel.Sel.Name
	switch pkg.Name {
	case "errors":
		return name == "New"
	case "fmt":
		return name == "Errorf"
	case "log":
		return strings.HasPrefix(name, "Print") || strings.HasPrefix(name, "Fatal") || strings.HasPrefix(name, "Panic")
	case "flag":
		return true
	case "http":
		return name == "Error"
	}
	return false
}

// TestNoSpanishMessages keeps logs, CLI help and error messages in English.
func TestNoSpanishMessages(t *testing.T) {
	var files []string
	for _, root := range []string{"../../internal", "."} {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(files) == 0 {
		t.Fatal("no Go files found")
	}
	fset := token.NewFileSet()
	for _, f := range files {
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !messageCall(call) {
				return true
			}
			for _, arg := range call.Args {
				// Literals anywhere in the argument, so "a" + "b" is covered.
				ast.Inspect(arg, func(n ast.Node) bool {
					lit, ok := n.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return true
					}
					s, err := strconv.Unquote(lit.Value)
					if err == nil && (spanishChars.MatchString(s) || spanishWords.MatchString(s)) {
						t.Errorf("%s: Spanish message %q", fset.Position(lit.Pos()), s)
					}
					return true
				})
			}
			return true
		})
	}
}
