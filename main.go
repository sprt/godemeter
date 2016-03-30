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

	"golang.org/x/tools/go/ast/astutil"
)

// Violation represents a violation of the Law of Demeter.
type Violation struct {
	Filename string
	Line     int
	Col      int
}

type astVisitor struct {
	info       *types.Info
	f          *ast.File
	fset       *token.FileSet
	violations []*Violation
}

func newAstVisitor(f *ast.File, fset *token.FileSet, info *types.Info) *astVisitor {
	return &astVisitor{
		info:       info,
		f:          f,
		fset:       fset,
		violations: []*Violation{},
	}
}

func (v *astVisitor) enclosingFuncDecl(expr ast.Node) *ast.FuncDecl {
	path, _ := astutil.PathEnclosingInterval(v.f, expr.Pos(), expr.End())
	for _, n := range path {
		// fmt.Printf("n: %#v\n", n)
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			return funcDecl
		}
	}
	return nil
}

func exprToIdent(expr ast.Expr) *ast.Ident {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident
	}
	return expr.(*ast.StarExpr).X.(*ast.Ident)
}

func (v *astVisitor) addViolation(expr *ast.CallExpr) {
	fpos := v.fset.Position(expr.Pos())
	violation := &Violation{
		Filename: fpos.Filename,
		Line:     fpos.Line,
		Col:      fpos.Column,
	}
	v.violations = append(v.violations, violation)
}

func (v *astVisitor) VisitCallExpr(callExpr *ast.CallExpr) (visitor ast.Visitor) {
	visitor = v

	fun, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		// Package-local function call
		return
	}

	switch call := v.info.ObjectOf(fun.Sel).(type) {
	case *types.Var:
		// TODO
	case *types.Func:
		callRecv := call.Type().(*types.Signature).Recv()
		if callRecv == nil {
			// Not a method call
			return
		}

		funcDecl := v.enclosingFuncDecl(fun)
		if funcDecl.Recv == nil {
			// Not inside a method
			return
		}

		if _, ok := fun.X.(*ast.CallExpr); ok {
			// Chained method call
			v.addViolation(callExpr)
			return
		}

		funcDeclRecv := funcDecl.Recv.List[0].Names[0]
		if callRecv.Name() == funcDeclRecv.Name {
			// Call on O itself
			return
		}

		x, sel := v.lookupXSel(fun)

		if funcDecl.Type.Params.NumFields() > 0 {
			for _, param := range funcDecl.Type.Params.List {
				name := param.Names[0].Name
				if name == sel.Name {
					// Call on one of m's parameters
					return
				}
			}
		}

		funcDeclScope := v.info.ObjectOf(funcDecl.Name).(*types.Func).Scope()
		if funcDeclScope.Lookup(sel.Name) != nil {
			// Call on object created in m
			// XXX: check if object *instantiated* in m, as opposed
			// to just declared
			return
		}

		for _, name := range exprToIdent(funcDecl.Recv.List[0].Type).Obj.Decl.(*ast.TypeSpec).Type.(*ast.StructType).Fields.List {
			if name.Names[0].Name == sel.Name {
				if x != nil && x.Name == funcDeclRecv.Name {
					// Call on one of O's direct components
					// XXX: check embedded methods
					return
				}
				break
			}
		}

		selVar := v.info.ObjectOf(sel).(*types.Var)
		if selVar.Pkg().Scope() == selVar.Parent() {
			// Call on global object
			return
		}

		v.addViolation(callExpr)
	}

	return
}

func (v *astVisitor) Visit(node ast.Node) ast.Visitor {
	if n, ok := node.(*ast.CallExpr); ok {
		return v.VisitCallExpr(n)
	}
	return v
}

func (v *astVisitor) lookupXSel(sexpr *ast.SelectorExpr) (retx, retsel *ast.Ident) {
	if ident, ok := sexpr.X.(*ast.Ident); ok {
		retsel = ident
		return
	}

	sexpr = sexpr.X.(*ast.SelectorExpr)
	retsel = sexpr.Sel

	ident, ok := sexpr.X.(*ast.Ident)
	if ok {
		retx = ident
	}
	for ; !ok; retx = sexpr.Sel {
		sexpr = sexpr.X.(*ast.SelectorExpr)
		_, ok = sexpr.X.(*ast.Ident)
	}

	return
}

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

	visitor := newAstVisitor(f, fset, info)
	ast.Walk(visitor, f)

	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}

	for _, violation := range visitor.violations {
		relpath, err := filepath.Rel(wd, violation.Filename)
		if err != nil {
			relpath = violation.Filename
		}
		fmt.Printf("%s:%d:%d\n", relpath, violation.Line, violation.Col)
	}
}
