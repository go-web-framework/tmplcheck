package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"os"
	"sync"

	tparse "text/template/parse"

	"golang.org/x/tools/go/loader"
)

// TODO: Support output formats: json, xml.

var (
	TemplatesPath string
	PackagePath   string
	LeftDelim     string
	RightDelim    string
)

func main() {
	flag.StringVar(&TemplatesPath, "t", "", "path to templates directory")
	flag.StringVar(&PackagePath, "p", "", "package import path")
	flag.StringVar(&LeftDelim, "ldelim", "{{", "left delimiter in templates")
	flag.StringVar(&RightDelim, "rdelim", "}}", "right delimiter in templates")

	flag.Parse()

	if TemplatesPath == "" {
		fmt.Fprintln(os.Stderr, "-t is required")
		os.Exit(2)
	}
	if PackagePath == "" {
		fmt.Fprintln(os.Stderr, "-p is required")
		os.Exit(2)
	}

	var wg sync.WaitGroup

	var usages map[string][]usage
	var identsForTemplateFile map[string][]templateIdents
	var err0, err1 error

	wg.Add(1)
	go func() {
		defer wg.Done()
		usages, err0 = parsePackage(PackagePath)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		identsForTemplateFile, err1 = parseTemplates(TemplatesPath)
	}()

	wg.Wait()

	// TODO: Implement decent way to show warnings.
	// fmt.Println(usages, err0)
	// fmt.Println(identsForTemplateFile, err1)

	var results []checkResult

	for k, v := range identsForTemplateFile {
		r := check(v, usages[fmt.Sprintf("%q", k)])
		r.TemplateFile = k
		results = append(results, r)
	}

	for _, r := range results {
		fmt.Println(&r)
	}
}

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
	TemplatePos tparse.Pos
	SourceFile  string
	SourcePos   int // TODO: not supported yet.
	Key         string
}

func (e *MissingError) Error() string {
	return fmt.Sprintf("%v: Missing key %q", e.TemplatePos, e.Key)
}

func containsString(slice []string, target string) bool {
	for _, str := range slice {
		if target == str {
			return true
		}
	}
	return false
}

func check(t []templateIdents, pkgUsages []usage) checkResult {
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
						TemplatePos: tident.Pos,
						Key:         s,
					})
				}
			}

		}
	}

	return res
}

// identValue returns the first value for the ident.
func identValue(a *ast.Ident) (string, error) {
	if a.Obj == nil || a.Obj.Decl == nil {
		return "", errors.New("failed to determine Decl")
	}
	vspec, ok := a.Obj.Decl.(*ast.ValueSpec)
	if !ok || len(vspec.Values) == 0 {
		return "", errors.New("unknown value")
	}
	return vspec.Values[0].(*ast.BasicLit).Value, nil
}

func compositeLitKeys(comp *ast.CompositeLit) []string {
	var ret []string
	for _, e := range comp.Elts {
		k := e.(*ast.KeyValueExpr).Key
		switch x := k.(type) {
		case *ast.BasicLit:
			ret = append(ret, x.Value) // map
		case *ast.Ident:
			ret = append(ret, x.Name) // struct
		}
	}
	return ret
}

func identToCompositeLit(id *ast.Ident) (*ast.CompositeLit, error) {
	asn, ok := id.Obj.Decl.(*ast.AssignStmt)
	if !ok || len(asn.Rhs) == 0 {
		return nil, errors.New("identToCompositeLit: wrong type")
	}

	cl, ok := asn.Rhs[0].(*ast.CompositeLit)
	if !ok {
		return nil, errors.New("identToCompositeLit: wrong type")
	}

	return cl, nil
}

type call interface {
	Type() []string
	Func() []string
	Handler(callexpr *ast.CallExpr) (name string, keys []string, err error)
}

var errUnsupportedArgs = errors.New("unsupported type for arguments")

var templatePackages = []call{
	&templatesSet{},
	&htmltemplateTemplate{}, // Order matters: Support html/template before text/template.
	&texttemplateTemplate{},
}

type templatesSet struct{}

func (t *templatesSet) Type() []string {
	return []string{"github.com/go-web-framework/templates.Set", "*github.com/go-web-framework/templates.Set"}
}

func (t *templatesSet) Func() []string { return []string{"Execute"} }

// Handler is the handler for templates.Set.
//
// Arguments support:
//
//   1. composite literals: Foo{X: Y}, map[KeyType]ValueType{x: y}
//   2. ident -> composite literal.
//
// If the composite literal is a map, only literal, in-place maps are supported. That is,
//
//   s.Execute(.., .., map[string]interface{} {
//     "qux": 2,
//     "bar": 10,
//   })
//
// is supported. But not:
//
//   m := map[string]interface{}{}
//   m["qux"] = 2
//   s.Execute(.., .., m)
//
func (t *templatesSet) Handler(callexpr *ast.CallExpr) (string, []string, error) {
	var name string
	var keys []string

	// Args[0] is the name of the template.

	switch a := callexpr.Args[0].(type) {
	case *ast.BasicLit:
		name = a.Value
	case *ast.Ident:
		n, err := identValue(a)
		if err != nil {
			return "", nil, err
		}
		name = n
	}

	// Args[2] is the arguments being passed.

	switch x := callexpr.Args[2].(type) {
	case *ast.CompositeLit:
		keys = compositeLitKeys(x)
	case *ast.Ident:
		c, err := identToCompositeLit(x)
		if err != nil {
			return "", nil, err
		}
		keys = compositeLitKeys(c)
	default:
		return "", nil, errUnsupportedArgs
	}

	return name, keys, nil
}

type htmltemplateTemplate struct{}

func (t *htmltemplateTemplate) Type() []string {
	return []string{"html/template.Template", "*html/template.Template"}
}

func (t *htmltemplateTemplate) Func() []string { return []string{"Execute"} }

func (t *htmltemplateTemplate) Handler(callexpr *ast.CallExpr) (string, []string, error) {
	return "", nil, errors.New("unsupported type")
}

type texttemplateTemplate struct{}

func (t *texttemplateTemplate) Type() []string {
	return []string{"text/template.Template", "*text/template.Template"}
}

func (t *texttemplateTemplate) Func() []string { return []string{"Execute"} }

func (t *texttemplateTemplate) Handler(callexpr *ast.CallExpr) (string, []string, error) {
	return "", nil, errors.New("unsupported type")
}

func match(typ, funcName string) (call, bool) {
	for _, tmpllib := range templatePackages {
		for _, t := range tmpllib.Type() {
			if t == typ {
				for _, f := range tmpllib.Func() {
					if f == funcName {
						return tmpllib, true
					}
				}
			}
		}
	}
	return nil, false
}

// usage represents a call to template with the keys.
//
// TODO: Include the file name and pos.
type usage struct {
	Template string   // name of template
	Keys     []string // keys passed to template
}

func parsePackage(path string) (map[string][]usage, error) {
	var conf loader.Config

	_, err := conf.FromArgs([]string{path}, false)
	if err != nil {
		return nil, err
	}

	prog, err := conf.Load()
	if err != nil {
		return nil, err
	}

	ourpkg := prog.Package(path)

	ret := make(map[string][]usage)
	var retErr error

	for _, f := range ourpkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				selexpr, ok := x.Fun.(*ast.SelectorExpr)
				if !ok {
					break
				}
				id, ok := selexpr.X.(*ast.Ident)
				if !ok {
					break
				}

				typ := ourpkg.TypeOf(id).String()
				funcName := selexpr.Sel.Name

				tl, ok := match(typ, funcName)
				if !ok {
					// Not a matching call. Move on to next call expression.
					break
				}

				name, keys, err := tl.Handler(x)
				if err != nil {
					retErr = err
					return false
				}
				ret[name] = append(ret[name], usage{name, keys})
			}

			return true
		})
	}

	return ret, retErr
}
