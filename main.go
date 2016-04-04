package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sprt/godemeter/demeter"
)

var path = "."

func main() {
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	finfo, err := os.Stat(path)
	if err != nil {
		log.Fatalln(err)
	}

	fun := demeter.AnalyzeFile
	if finfo.IsDir() {
		fun = demeter.AnalyzePackage
	}

	violations, err := fun(path)
	if err != nil {
		log.Fatalln(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}

	for _, violation := range violations {
		relpath, err := filepath.Rel(wd, violation.Filename)
		if err != nil {
			relpath = violation.Filename
		}
		fmt.Printf("%s:%d:%d: %s\n", relpath, violation.Line, violation.Col, violation.Expr)
	}
}
