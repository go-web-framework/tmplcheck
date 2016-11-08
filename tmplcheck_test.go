package main

import (
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// runTest is intended to be the testing equivalent of mainImpl.
func runTest(ppath, tpath string) []checkResult {
	PackagePath, TemplatesPath = ppath, tpath
	checkArgs()
	return DoAll()
}

func TestTmplCheck(t *testing.T) {
	const ppath = "github.com/go-web-framework/tmplcheck/testdata/src"
	const tpath = filepath.Join("testdata", "templates")

	Convey("tmplcheck", t, func() {
		Convey("nil", func() {
			expected := []checkResult{}
			results := runTest(ppath, tpath)
		})
	})
}
