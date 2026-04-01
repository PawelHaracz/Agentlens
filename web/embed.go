package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded dist filesystem.
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
