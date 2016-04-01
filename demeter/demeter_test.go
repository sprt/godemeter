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

func TestNoViolation(t *testing.T) {
	inputs := make(map[string]string)

	inputs["Empty package"] = `
	package main`

	inputs["Call on O itself"] = `
	package main
	type Foo struct{}
	func (f *Foo) foo() {}
	func (f *Foo) bar() { f.foo() }`

	inputs["Call on O itself (embedded object's method)"] = `
	package main
	type Foo struct{}
	func (f *Foo) foo() {}
	type Bar struct{ *Foo }
	func (b *Bar) bar() { b.foo() }`

	inputs["Call on m's parameter"] = `
	package main
	type Foo struct{}
	func (f *Foo) foo() {}
	type Bar struct{}
	func (b *Bar) bar(f *Foo) { f.foo() }`

	inputs["Call on object created in m"] = `
	package main
	type Foo struct{}
	func (f *Foo) foo() {}
	type Bar struct{}
	func (b *Bar) bar() { f := &Foo{}; f.foo() }`

	inputs["Call on O's direct component"] = `
	package main
	type Foo struct{}
	func (f *Foo) foo() {}
	type Bar struct{ f *Foo }
	func (b *Bar) bar() { b.f.foo() }`

	inputs["Call on global object"] = `
	package main
	type Foo struct{}
	func (f *Foo) foo() {}
	var f = &Foo{}
	type Bar struct{}
	func (b *Bar) bar() { f.foo() }`

	for name, s := range inputs {
		violations, err := analyzeString(s)
		if err != nil {
			t.Errorf("error: %v", err)
		}
		if len(violations) != 0 {
			t.Errorf("%s: got %#v, expected no violations", name, violations)
		}
	}
}

func TestCallOnObjectDeclaredInAnotherMethod(t *testing.T) {
	s := `
	package main
	type Foo struct{}
	func (f *Foo) foo() {}
	type Bar struct{}
	func (b *Bar) foo() *Foo { return &Foo{} }
	func (b *Bar) bar() { f := b.foo(); f.foo() }`

	violations, err := analyzeString(s)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if len(violations) != 1 || violations[0].Line != 7 {
		t.Errorf("got %#v, expected 1 violation at line 7", violations)
	}
}
