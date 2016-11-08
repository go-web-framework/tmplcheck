package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/go-web-framework/templates"
)

type Foo struct {
	Name  string
	Title string
	FF    string
}

type League struct {
	Teams   []string // Team names.
	Founded int      // Year founded.
}

func main() {
	set := &templates.Set{DefaultArgs: templates.Args{"Title": "default title"}}
	err := set.Parse(filepath.Join("..", "templates"))
	if err != nil {
		log.Fatalln(err)
	}

	err = set.Execute("root.html", os.Stdout, nil)
	if err != nil {
		log.Fatalln(err)
	}
}
