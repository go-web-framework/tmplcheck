package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
)

type MissingError struct {
	Usage         Usage
	TemplateIdent TemplateIdent
	MissingKey    string
}

func (e MissingError) MarshalJSON() ([]byte, error) {
	type t struct {
		Path string `json:"file"`
		Line int    `json:"line"`
		Col  int    `json:"col"`
	}

	type s struct {
		Path       string `json:"file"`
		Line       int    `json:"line"`
		Key        string `json:"key"`
		MethodCall string `json:"call"`
	}

	aux := struct {
		Template t `json:"template"`
		Source   s `json:"source"`
	}{
		t{
			e.TemplateIdent.Path,
			e.TemplateIdent.Line,
			e.TemplateIdent.Col,
		},
		s{
			e.Usage.Path,
			e.Usage.Line,
			e.MissingKey,
			e.Usage.Obj + "." + e.Usage.Call,
		},
	}

	return json.Marshal(aux)
}

func (e MissingError) String() string {
	return fmt.Sprintf(
		"%d:%d: uses %q, but %s:%d: %s.%s is missing %q",
		e.TemplateIdent.Line, e.TemplateIdent.Col,
		e.MissingKey, e.Usage.Path, e.Usage.Line, e.Usage.Obj, e.Usage.Call, e.MissingKey,
	)
}

type checkResult struct {
	Template string         `json:"template"` // path of template file
	Errs     []MissingError `json:"missing"`
}

func (c checkResult) String() string {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%s\n", c.Template))
	for i, e := range c.Errs {
		buf.WriteString(fmt.Sprintf("%s", e))
		if i != len(c.Errs)-1 {
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

// DoAll returns the results of checking the source files against the
// template files. The flag global variables are expected to be set when
// DoAll is called.
func DoAll() []checkResult {
	usages, identsForTemplate, err := goParseAll(PackagePath, TemplatesPath)
	if err != nil {
		exitErr(err)
	}
	return doCheck(usages, identsForTemplate)
}

func goParseAll(ppath, tpath string) (map[string][]Usage, map[string][]TemplateIdent, error) {
	var wg sync.WaitGroup

	var usages map[string][]Usage
	var identsForTemplate map[string][]TemplateIdent
	var err0, err1 error

	wg.Add(1)
	go func() {
		defer wg.Done()
		usages, err0 = parsePackage(ppath)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		identsForTemplate, err1 = parseTemplates(tpath)
	}()

	wg.Wait()

	if err0 != nil {
		return nil, nil, err0
	}
	if err1 != nil {
		return nil, nil, err1
	}

	return usages, identsForTemplate, nil
}

// doCheck compares the usages (in go source) with the identifiers used in
// templates. One checkResult for each template is returned.
func doCheck(usages map[string][]Usage, identsForTemplate map[string][]TemplateIdent) []checkResult {
	var results []checkResult

	for k, v := range identsForTemplate {
		u := usages[k]
		r := check(v, u)
		r.Template = k
		results = append(results, r)
	}

	return results
}

func check(t []TemplateIdent, pkgUsages []Usage) checkResult {
	// For every identifier in a template, every usage/call to the template
	// should contain that identifier. In other words, for every identifier
	// in a template, if there is any usage/call to the template not containing the
	// identifier then we should emit a warning or error.

	res := checkResult{}

	for _, tident := range t {
		for _, s := range tident.Idents {

			for _, u := range pkgUsages {
				if containsString(u.Keys, s) {
					continue
				} else {
					res.Errs = append(res.Errs, MissingError{
						Usage:         u,
						TemplateIdent: tident,
						MissingKey:    s,
					})
				}
			}

		}
	}

	return res
}
