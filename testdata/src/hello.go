package main

import (
	"log"
	"os"

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
	templateSet := &templates.Set{}
	err := templateSet.Parse("../templates")
	if err != nil {
		log.Fatalln(err)
	}

	//////////////////////////////////////////////////////////////////////////////////////

	err = templateSet.Execute("hello.html", os.Stdout, Foo{FF: "", Name: "Alice"})
	if err != nil {
		log.Fatalln(err)
	}

	//////////////////////////////////////////////////////////////////////////////////////

	p := Foo{Name: "Bob", Title: "Yay!"}
	err = templateSet.Execute("hello.html", os.Stdout, p)
	if err != nil {
		log.Fatalln(err)
	}
	err = templateSet.Execute("world.html", os.Stdout, p)
	if err != nil {
		log.Fatalln(err)
	}

	//////////////////////////////////////////////////////////////////////////////////////

	l := League{
		Teams:   []string{"Liverpool", "Southhampton", "Chelsea"},
		Founded: 1880,
	}
	err = templateSet.Execute("hello.html", os.Stdout, l)
	if err != nil {
		log.Fatalln(err)
	}

	//////////////////////////////////////////////////////////////////////////////////////

	err = templateSet.Execute("hello.html", os.Stdout, map[string]interface{}{"Title": 3})
	if err != nil {
		log.Fatalln(err)
	}

	//////////////////////////////////////////////////////////////////////////////////////

	m := map[string]interface{}{}
	m["Crazy"] = 4
	err = templateSet.Execute("root.html", os.Stdout, m)
	if err != nil {
		log.Fatalln(err)
	}
}
