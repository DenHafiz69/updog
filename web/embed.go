package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html style.css app.js
var staticFiles embed.FS

func FS() fs.FS {
	return staticFiles
}
