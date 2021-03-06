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

	abspath, err := filepath.Abs(path)
	if err != nil {
		log.Fatalln(err)
	}

	violations, err := fun(abspath)
	if err != nil {
		log.Fatalln(err)
	}

	for _, v := range violations {
		fmt.Println(v)
	}
}
