package webassets

import (
	"embed"
	"io/fs"
)

// dist is populated by `make web-assets`. A tracked placeholder keeps the
// package buildable in source-only test runs; production builds verify that an
// index.html is present before compiling the web binary.
//
//go:embed all:dist
var embedded embed.FS

func FS() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
