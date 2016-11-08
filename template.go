package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	htemplate "html/template"
	tparse "text/template/parse"
)

/*
NOTES
=====

From the text/template package comment: "Actions"--data evaluations or control
structures--are delimited by "{{" and "}}".

From the documentation for text/template/parse:

Pipeline nodes (type PipeNode) contains []*VariableNode and []*CommandNode, which are
of interest to us to determine fields in use in a template.
Also see: type FieldNode and type ChainNode.

The following type contain a PipeNode directly:

  * ActionNode
  * BranchNode
  * TemplateNode

Other types contain or embed one of the above 3 types, thus indirectly containing a PipeNode:

  * IfNode
  * RangeNode
  * WithNode

ListNode is used for traversals. In addition to the Tree root, it is present in BranchNode.
*/

// templateIdent is identifiers and their position in the file.
type templateIdent struct {
	Path string     // Relative path of identifier's file
	Pos  tparse.Pos // Byte position in file
	Line int        // Line number in file
	Col  int        // Column in file

	// Idents is the identifiers at the location. It is a
	// list to account for cases such as FieldNode in which
	// identitifers are chained.
	Idents []string
}

func lineCol(byteOffset int, lines [][]byte) (line int, col int) {
	prevTotal := -1
	total := 0
	line = 1

	for _, l := range lines {
		prevTotal = total
		total += len(l) + 1 // + 1 for \n
		if total >= byteOffset {
			break
		}
		line++
	}

	if prevTotal != -1 {
		col = byteOffset - prevTotal
	} else {
		col = byteOffset
	}

	return
}

func parseTemplate(b []byte, relpath string) ([]templateIdent, error) {
	const someName = ""
	t, err := htemplate.New(someName).Delims(LeftDelim, RightDelim).Parse(string(b))
	if err != nil {
		return nil, err
	}

	var ret []templateIdent

	// TODO(nishanths): Does Windows line-ending need
	// different handling?
	// TODO(nishanths): unimportant: Perform incremental search since
	// we are only moving forward thru the file.
	lines := bytes.Split(b, []byte("\n"))

	err = Walk(t.Tree.Root, func(node tparse.Node) error {
		switch n := node.(type) {
		case *tparse.IdentifierNode:
			// XXX(nishanths): According to tparse doc, NodeIdentifier is
			// always a function name. We ignore these because we only use
			// default functions when parsing templates.
			// In fact, if any non-default function exists in the template
			// Parse will fail because of unknown identifier.
			// Will need change when we support template.FuncMap.
			if n.NodeType == tparse.NodeIdentifier {
				break
			}
			l, c := lineCol(int(n.Pos), lines)
			ret = append(ret, templateIdent{
				Path:   relpath,
				Pos:    n.Pos,
				Line:   l,
				Col:    c,
				Idents: []string{n.Ident},
			})
		case *tparse.FieldNode:
			l, c := lineCol(int(n.Pos), lines)
			ret = append(ret, templateIdent{
				Path:   relpath,
				Pos:    n.Pos,
				Line:   l,
				Col:    c,
				Idents: n.Ident,
			})
		default:
			// This branch is purely for documenting the source code:
			// we do not care about types besides the above.
		}
		return nil
	})

	return ret, err
}

func parseTemplates(root string) (map[string][]templateIdent, error) {
	ret := make(map[string][]templateIdent)

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

		idents, err := parseTemplate(b, relp)
		if err != nil {
			return err
		}
		ret[relp] = idents
		return nil
	})

	return ret, err
}

type WalkFunc func(node tparse.Node) error

func walk(root tparse.Node, fx WalkFunc, incomingErr error) error {
	if incomingErr != nil {
		return incomingErr
	}

	var err error

	// XXX(nishanths): panic is okay for now because this is a main
	// program. We expect that the paths with panic calls will never be reached,
	// but would be nice to know of cases when it happens.

	// NOTE(nishanths): The err = funccall(.., err) style with the incoming
	// error check means that we do not have to check err != nil each
	// time (helps readability). However, this has the added overhead of the
	// function calls even after a non-nil error has been detected.

	switch n := root.(type) {
	case nil:
		// Nothing to do.
		// TODO(nishanths): Does this case ever happen?
	case *tparse.ListNode:
		for _, n := range n.Nodes {
			err = walk(n, fx, err)
		}
	case *tparse.ActionNode:
		err = fx(n)
		err = walk(n.Pipe, fx, err)
	case *tparse.BoolNode:
		err = fx(n)
	case *tparse.BranchNode:
		// This is not a concrete node type that will be encountered.
		// See IfNode, WithNode, and RangeNode instead.
		panic("tmplcheck: expected BranchNode to not be a concrete node type")
	case *tparse.ChainNode:
		err = fx(n)
		err = walk(n.Node, fx, err)
	case *tparse.CommandNode:
		err = fx(n)
		for _, a := range n.Args {
			err = walk(a, fx, err)
		}
	case *tparse.DotNode:
		err = fx(n)
	case *tparse.FieldNode:
		err = fx(n)
	case *tparse.IdentifierNode:
		err = fx(n)
	case *tparse.IfNode:
		err = fx(n)
		err = walk(n.Pipe, fx, err)
		err = walk(n.List, fx, err)
		if n.ElseList != nil {
			err = walk(n.ElseList, fx, err)
		}
	case *tparse.NilNode:
		err = fx(n)
	case *tparse.NumberNode:
		err = fx(n)
	case *tparse.PipeNode:
		err = fx(n)
		for _, v := range n.Decl {
			err = walk(v, fx, err)
		}
		for _, v := range n.Cmds {
			err = walk(v, fx, err)
		}
	case *tparse.RangeNode:
		err = fx(n)
		err = walk(n.Pipe, fx, err)
		err = walk(n.List, fx, err)
		if n.ElseList != nil {
			err = walk(n.ElseList, fx, err)
		}
	case *tparse.StringNode:
		err = fx(n)
	case *tparse.TemplateNode:
		err = fx(n)
		err = walk(n.Pipe, fx, err)
	case *tparse.TextNode:
		err = fx(n)
	case *tparse.VariableNode:
		err = fx(n)
	case *tparse.WithNode:
		err = fx(n)
		err = walk(n.Pipe, fx, err)
		err = walk(n.List, fx, err)
		if n.ElseList != nil {
			err = walk(n.ElseList, fx, err)
		}
	default:
		panic("tmplcheck: encountered unknown node type")
	}

	return err
}

func Walk(node tparse.Node, fx WalkFunc) error {
	return walk(node, fx, nil)
}
