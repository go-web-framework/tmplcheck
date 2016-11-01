package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	htemplate "html/template"
	tparse "text/template/parse"
)

/*

NOTES

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

func parseTemplates(root string) (map[string][]templateIdents, error) {
	ret := make(map[string][]templateIdents)

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

	switch n := root.(type) {
	case nil:
		// Nothing to do.
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
		panic("tmplcheck: DEBUG: BranchNode is not a concrete node type")
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
		panic("tmplcheck: DEBUG: encountered unknown node type")
	}

	return err
}

func Walk(node tparse.Node, fx WalkFunc) error {
	return walk(node, fx, nil)
}

type templateIdents struct {
	File   string
	Pos    tparse.Pos
	Idents []string
}

func parseTemplate(b []byte, relpath string) ([]templateIdents, error) {
	const someMiscName = ""
	t, err := htemplate.New(someMiscName).Delims(LeftDelim, RightDelim).Parse(string(b))
	if err != nil {
		return nil, err
	}

	var ret []templateIdents

	err = Walk(t.Tree.Root, func(node tparse.Node) error {
		switch n := node.(type) {
		case *tparse.IdentifierNode:
			ret = append(ret, templateIdents{relpath, n.Pos, []string{n.Ident}})
		case *tparse.FieldNode:
			ret = append(ret, templateIdents{relpath, n.Pos, n.Ident})
		}

		return nil
	})

	return ret, err
}
