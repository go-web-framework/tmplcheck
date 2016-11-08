# tmplcheck

[![wercker status](https://app.wercker.com/status/733965ba5063a29ddf1aa17556713f7e/s/master "wercker status")](https://app.wercker.com/project/byKey/733965ba5063a29ddf1aa17556713f7e)

tmplcheck checks that templates do not use keys not passed in from Go source code in Execute calls.

Currently only supports `github.com/go-web-framework/templates`. Future support for `html/template` and `text/template` is planned.

tmplcheck statically analyzes your source code and templates to help prevent panics at run time such as:

```
panic: template: test:1:2: executing "test" at <.Cnt>: can't evaluate field Cnt in type main.Inventory
```

Work in progress. See TODOs in source code.

## Install

```
go get -u github.com/go-web-framework/tmplcheck
``` 

## Quick Start

```sh
tmplcheck \
    -p <import path of go code> \
    -t <path to templates> \
    -format <plain|json>
```

See `tmplcheck -help` for more.

## Example

`tmplcheck` would output the following for the files below:

```
5:13: uses "Title", but hello.go:29: set.Execute is missing "Title"
8:14: uses "X", but hello.go:29: set.Execute is missing "X"
8:17: uses "Y", but hello.go:29: set.Execute is missing "Y"
```

Template `root.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>{{.Title}}</title>
</head>
<body>
    {{ if and .X .Y }}
       xy
    {{ end }}
</body>
</html>
```

And corresponding Go source code in `hello.go` that executes `root.html`:

```go
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/go-web-framework/templates"
)

func main() {
	set := &templates.Set{}
	err := set.Parse(filepath.Join("..", "templates"))
	if err != nil {
		log.Fatalln(err)
	}

	err = set.Execute("root.html", os.Stdout, nil)
	if err != nil {
		log.Fatalln(err)
	}
}
```

## License 

MIT. See the LICENSE file at the root of the repo.