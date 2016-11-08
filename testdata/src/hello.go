package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/go-web-framework/templates"
)

func main() {
	set := &templates.Set{}
	err := set.Parse(filepath.Join("..", "templates"))
	if err != nil {
		log.Fatalln(err)
	}

	err = set.Execute("root.html", os.Stdout, nil)
	if err != nil {
		log.Fatalln(err)
	}
}
