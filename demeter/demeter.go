package demeter

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/ast/astutil"
)

// Violation represents a violation of the Law of Demeter.
type Violation struct {
	Filename string
	Line     int
	Col      int
	Expr     string
}

// AnalyzeFile analyzes a single file and returns the violations.
func AnalyzeFile(filename string) ([]*Violation, error) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return nil, err
	}

	return analyzeFiles(filepath.Dir(filename), []*ast.File{f}, fset)
}

// AnalyzePackage analyzes a package and returns the violations.
func AnalyzePackage(dirname string) ([]*Violation, error) {
	fset := token.NewFileSet()

	packages, err := parser.ParseDir(fset, dirname, nil, 0)
	if err != nil {
		return nil, err
	}

	violations := []*Violation{}
	for _, p := range packages {
		files := []*ast.File{}
		for _, f := range p.Files {
			files = append(files, f)
		}
		v, err := analyzeFiles(dirname, files, fset)
		if err != nil {
			return nil, err
		}
		violations = append(violations, v...)
	}

	return violations, nil
}

func analyzeFiles(packagePath string, files []*ast.File, fset *token.FileSet) ([]*Violation, error) {
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

	_, err := config.Check(packagePath, fset, files, info)
	if err != nil {
		return nil, err
	}

	violations := []*Violation{}
	for _, f := range files {
		visitor := newAstVisitor(f, fset, info)
		ast.Walk(visitor, f)
		violations = append(violations, visitor.violations...)
	}

	return violations, nil
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

func (v *astVisitor) Visit(node ast.Node) ast.Visitor {
	if n, ok := node.(*ast.CallExpr); ok {
		return v.visitCallExpr(n)
	}
	return v
}

func (v *astVisitor) visitCallExpr(callExpr *ast.CallExpr) (visitor ast.Visitor) {
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
			//
			// Local-function calls aren't selector expressions,
			// but external ones are,
			// which is why we're checking for a receiver.
			return
		}

		funcDecl := v.enclosingFuncDecl(fun)
		if funcDecl == nil || funcDecl.Recv == nil {
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

		if sel.Obj != nil {
			if assignStmt, ok := sel.Obj.Decl.(*ast.AssignStmt); ok {
				if rhs, ok := assignStmt.Rhs[0].(*ast.CallExpr); ok {
					if fun, ok := rhs.Fun.(*ast.Ident); !ok || fun.Name != "new" {
						// Call on object initialized in m
						// but instantiated elsewere
						v.addViolation(callExpr)
						return
					}
				}
			}
		}

		funcDeclScope := v.info.ObjectOf(funcDecl.Name).(*types.Func).Scope()
		if funcDeclScope.Lookup(sel.Name) != nil {
			// Call on object created in m
			return
		}

		// TODO: test this with inter-package structs
		if recvType, ok := v.info.TypeOf(exprToIdent(funcDecl.Recv.List[0].Type)).Underlying().(*types.Struct); ok {
			for i := 0; i < recvType.NumFields(); i++ {
				if recvType.Field(i).Name() == sel.Name {
					if x != nil && x.Name == funcDeclRecv.Name {
						// Call on one of O's direct components
						return
					}
					break
				}
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

func (v *astVisitor) addViolation(expr *ast.CallExpr) {
	fpos := v.fset.Position(expr.Pos())

	var buf bytes.Buffer
	printer.Fprint(&buf, v.fset, expr.Fun)

	violation := &Violation{
		Filename: fpos.Filename,
		Line:     fpos.Line,
		Col:      fpos.Column,
		Expr:     buf.String(),
	}
	v.violations = append(v.violations, violation)
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

func exprToIdent(expr ast.Expr) *ast.Ident {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident
	}
	return expr.(*ast.StarExpr).X.(*ast.Ident)
}
