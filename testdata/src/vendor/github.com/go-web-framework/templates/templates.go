// Package templates improves the html/template package
// by supporting parsing directories, automatically associate
// templates with packages, and specifying default arguments
// during template execution.
//
// The Set type provides this functionality. A Set can be safely
// executed concurrently.
package templates

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const defineStart = "{{define %q}}"
const defineEnd = "{{end}}"

// Set is a collection of templates.
type Set struct {
	// Funcs is the functions available to all templates.
	// If nil, no functions will be available.
	// The field has to be set before Parse is called.
	Funcs template.FuncMap

	// PartialsDir is the directory in which partials are stored.
	// In html/template terminology, this is the content in between
	// {{define "foo"}} and {{end}} that will be executed by
	// using {{template "foo" .}} from a parent template.
	PartialsDir string

	// DefaultArgs is the arguments available in all templates.
	// These values are not used if the args specified in Execute
	// are non-nil or not of type Args.
	// If this field is nil, only the args directly specified in Execute
	// calls are used.
	DefaultArgs Args

	// Templates is the map of all top-level templates. Partials
	// are not included in this map. Typically, users will not use
	// this directly but instead call Execute.
	Templates map[string]*template.Template

	// LDelim and RDelim are the delimiters to use when parsing templates.
	// This is the same as the argument to the Delims method in
	// html/template.
	// The fields have to be set before Parse is called. If empty, the
	// defaults "{{" and "}}" are used.
	LDelim, RDelim string
}

// Args is the arguments available to templates upon execution.
type Args map[string]interface{}

var ErrNoSuchTemplate = errors.New("templates: no matching template for name")

func (s *Set) execute(name string, w io.Writer, args interface{}) error {
	t, ok := s.Templates[name]
	if !ok {
		return ErrNoSuchTemplate
	}

	if args == nil {
		args = s.DefaultArgs
	} else if m, ok := args.(Args); ok {
		args = normalize(s.DefaultArgs, m)
	}

	return t.Execute(w, args)
}

// Execute executes the template for the given name using the given args.
// If args is of type Args, args is merged with s.DefaultArgs before
// executing the template.
//
// If a template with the specified name does not exist, ErrNoSuchTemplate
// is returned.
func (s *Set) Execute(name string, w io.Writer, args interface{}) error {
	return s.execute(name, w, args)
}

func normalize(def, new Args) Args {
	var ret Args

	for k, v := range def {
		if ret == nil {
			ret = make(Args)
		}
		ret[k] = v
	}

	for k, v := range new {
		if ret == nil {
			ret = make(Args)
		}
		ret[k] = v
	}

	return ret
}

func delims(left, right string) (string, string) {
	if left == "" {
		left = "{{"
	}
	if right == "" {
		right = "}}"
	}
	return left, right
}

// Parse parses the directory specified by path. The partials in
// s.PartialsDir will be parsed and associated with each of the parsed
// templates. s.PartialsDir should be a top-level subdirectory of path.
// If s.PartialsDir is empty, no partials assocation is performed.
func (s *Set) Parse(path string) error {
	m, err := readDir(path)
	if err != nil {
		return err
	}

	// Standardize path separators,
	for k, v := range m {
		delete(m, k)
		m[filepath.ToSlash(k)] = v
	}

	partials := make(map[string]string)

	for k, v := range m {
		if strings.HasPrefix(k, s.PartialsDir+"/") {
			partials[k] = string(v)
		}
	}

	ldelim, rdelim := delims(s.LDelim, s.RDelim)

	// The partials should be parsed with each main template
	// to be available in the main template.
	for k, v := range m {
		if strings.HasPrefix(k, s.PartialsDir+"/") {
			continue
		}

		templ, err := template.New(k).Funcs(s.Funcs).Delims(ldelim, rdelim).Parse(string(v))
		if err != nil {
			return err
		}

		for name, contents := range partials {
			if _, err := templ.Funcs(s.Funcs).Delims(ldelim, rdelim).
				Parse(fmt.Sprintf(defineStart, name) + contents + defineEnd); err != nil {
				return err
			}
		}

		if s.Templates == nil {
			s.Templates = make(map[string]*template.Template)
		}
		s.Templates[k] = templ
	}

	return nil
}

func readDir(root string) (map[string][]byte, error) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, err
	}

	var m map[string][]byte // Lazily initialized in Walk.

	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		b, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}

		relp, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}

		if m == nil {
			m = make(map[string][]byte)
		}
		m[relp] = b

		return nil
	})

	return m, err
}
