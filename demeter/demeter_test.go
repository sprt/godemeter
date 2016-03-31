package demeter

import (
	"go/parser"
	"go/token"
	"log"
	"testing"
)

const filename = "main.go"

func analyzeString(s string) ([]*Violation, error) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, filename, s, 0)
	if err != nil {
		log.Fatalln("analyzeString:", err)
	}

	return analyzeFile(filename, f, s, fset)
}

func TestEmptyPackage(t *testing.T) {
	s := "package main"

	violations, err := analyzeString(s)
	if err != nil {
		t.Errorf("TestEmptyPackage: error: %v", err)
	}

	if len(violations) != 0 {
		t.Errorf("TestEmptyPackage: got %#v, expected empty slice", violations)
	}
}
