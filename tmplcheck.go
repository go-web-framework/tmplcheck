package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/loader"
)

var (
	TemplatesPath string
	PackagePath   string
	LeftDelim     string
	RightDelim    string
	OutputFormat  string
)

func main() {
	flag.StringVar(&TemplatesPath, "t", "", "path to templates directory")
	flag.StringVar(&PackagePath, "p", "", "package import path")
	flag.StringVar(&LeftDelim, "ldelim", "", "left delimiter in templates")
	flag.StringVar(&RightDelim, "rdelim", "", "right delimiter in templates")
	flag.StringVar(&OutputFormat, "format", "", "output format (plain,json)")
	flag.Parse()

	mainImpl()
}

// checkArgs checks that required arguments are provided
// and fills in defaults for others. Defaults are filled
// in here and not using flags package to help with testing.
func checkArgs() {
	if TemplatesPath == "" {
		exitErr("-t is required")
	}

	if PackagePath == "" {
		exitErr("-p is required")
	}

	if LeftDelim == "" {
		LeftDelim = "{{"
	}

	if RightDelim == "" {
		RightDelim = "}}"
	}

	if OutputFormat == "" {
		OutputFormat = "plain"
	}
}

func exitErr(v interface{}) {
	fmt.Fprintln(os.Stderr, v)
	os.Exit(1)
}

func mainImpl() {
	checkArgs()
	results := DoAll()
	// TODO(nishanths): Refactor into function and use OutputFormat.
	for _, r := range results {
		fmt.Println(&r)
	}
}

func containsString(slice []string, target string) bool {
	for _, str := range slice {
		if target == str {
			return true
		}
	}
	return false
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
	// Type are the names of all types that are supported.
	Type() []string

	// Func are the names of the function that are supported.
	Func() []string

	// Handler returns the template name and the keys used inside
	// the template.
	Handler(callexpr *ast.CallExpr) (name string, keys []string, err error)
}

var errUnsupportedArgs = errors.New("unsupported type for arguments")

var supportedTemplatePackages = []call{
	&templatesSet{},
	&htmltemplateTemplate{}, // Order matters: analyze for html/template before text/template.
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
		// nil, variable name
		if x.Name == "nil" {
			break
		}
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

func doesMatch(typ, funcName string) (call, bool) {
	for _, tmpllib := range supportedTemplatePackages {
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

// usage represents a call to execute a template with the keys.
//
// TODO: Include the file name and pos.
type usage struct {
	Path string    // path of go source file
	Pos  token.Pos // byte position of the method call in the go source file.
	Line int

	// TODO: Col is not supported yet. But not important
	// since method calls are generally long lines, so Col is not that great.
	// Col  int

	Obj  string // object on which method is called
	Call string // called method name

	Template string   // name of template being executed
	Keys     []string // keys passed to template
}

func parsePackage(path string) (map[string][]usage, error) {
	var conf loader.Config

	// TODO: create from filenames to support source file name.
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

				tl, ok := doesMatch(typ, funcName)
				if !ok {
					// Not a matching call. Move on to next call expression.
					break
				}

				name, keys, err := tl.Handler(x)
				if err != nil {
					retErr = err
					return false
				}

				file := prog.Fset.File(x.Fun.Pos())
				ret[name] = append(ret[name], usage{
					Path: filepath.Base(file.Name()),
					Pos:  x.Fun.Pos(),
					Line: file.Line(x.Fun.Pos()),

					Obj:  id.Name,
					Call: funcName,

					Template: name,
					Keys:     keys,
				})
			}

			return true
		})
	}

	return ret, retErr
}
