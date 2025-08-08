package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var staticFiles embed.FS

// GetStaticFileSystem returns the embedded static file system
func GetStaticFileSystem() http.FileSystem {
	// Get the web subdirectory from the embedded filesystem
	webFS, err := fs.Sub(staticFiles, "web")
	if err != nil {
		panic("failed to create web filesystem: " + err.Error())
	}

	return http.FS(webFS)
}
