package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"

	"github.com/sprt/godemeter/demeter"
)

var path = ""

func main() {
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		log.Fatalln(err)
	}

	info := &types.Info{
		// TODO: check if we can remove any
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Implicits:  make(map[ast.Node]types.Object),
	}

	config := &types.Config{
		Error: func(err error) {
			fmt.Println(err)
		},
		Importer: importer.Default(),
	}

	_, err = config.Check(path, fset, []*ast.File{f}, info)
	if err != nil {
		log.Fatalln(err)
	}

	visitor := demeter.NewAstVisitor(f, fset, info)
	ast.Walk(visitor, f)

	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}

	for _, violation := range visitor.Violations {
		relpath, err := filepath.Rel(wd, violation.Filename)
		if err != nil {
			relpath = violation.Filename
		}
		fmt.Printf("%s:%d:%d\n", relpath, violation.Line, violation.Col)
	}
}
