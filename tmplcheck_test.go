package main

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// runTest is intended to be the testing equivalent of mainImpl.
func runTest(ppath, tpath string) []checkResult {
	PackagePath, TemplatesPath = ppath, tpath
	OutputFormat = "json"
	checkArgs()
	return DoAll()
}

func TestTmplCheck(t *testing.T) {
	ppath := "github.com/go-web-framework/tmplcheck/testdata/src"
	tpath := filepath.Join("testdata", "templates")

	Convey("tmplcheck", t, func() {
		Convey("nil", func() {
			b, err := ioutil.ReadFile(filepath.Join("testdata", "expected", "nil0.json"))
			So(err, ShouldBeNil)
			buf := bytes.Buffer{}
			output(&buf, runTest(ppath, tpath))
			So(buf.String(), ShouldEqual, string(b))
		})
	})
}
