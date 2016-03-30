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

	var fun func(string) ([]*demeter.Violation, error)
	if finfo.IsDir() {
		fun = demeter.AnalyzeDir
	} else {
		fun = demeter.AnalyzeFile
	}

	violations, err := fun(path)
	if err != nil {
		log.Fatalln(path)
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
		fmt.Printf("%s:%d:%d\n", relpath, violation.Line, violation.Col)
	}
}
