package main

import (
	"bytes"
	"fmt"
	"go/token"
	"sync"
)

type checkResult struct {
	TemplateFile string
	Errs         []error
}

func (c checkResult) String() string {
	buf := bytes.Buffer{}
	for _, e := range c.Errs {
		buf.WriteString(fmt.Sprintf("%s: %v\n", c.TemplateFile, e))
	}
	return buf.String()
}

type MissingError struct {
	SourcePos token.Pos // TODO: not supported yet.
	Key       string
	Call      string // TODO: Also get object name.
}

func (e *MissingError) Error() string {
	return fmt.Sprintf(
		"xx: Missing key %q in call %q at <filename>:%v",
		e.Key, e.Call, e.SourcePos,
	)
}

// DoAll returns the results of checking the source files against the
// template files. The flag global variables are expected to be set when
// DoAll is called.
func DoAll() []checkResult {
	usages, identsForTemplate, err := goParseAll(PackagePath, TemplatesPath)
	if err != nil {
		exitErr(err)
	}
	fmt.Println(usages)
	fmt.Println(identsForTemplate)

	return doCheck(usages, identsForTemplate)
}

func goParseAll(ppath, tpath string) (map[string][]usage, map[string][]templateIdent, error) {
	var wg sync.WaitGroup

	var usages map[string][]usage
	var identsForTemplate map[string][]templateIdent
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

func doCheck(usages map[string][]usage, identsForTemplate map[string][]templateIdent) []checkResult {
	var results []checkResult

	for k, v := range identsForTemplate {
		u := usages[fmt.Sprintf("%q", k)]
		r := check(v, u)
		r.TemplateFile = k
		results = append(results, r)
	}

	return results
}

func check(t []templateIdent, pkgUsages []usage) checkResult {
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
					res.Errs = append(res.Errs, &MissingError{
						Key:       s,
						SourcePos: u.Pos,
						Call:      u.Call,
					})
				}
			}

		}
	}

	return res
}
