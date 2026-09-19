package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed studio/*
var studioFS embed.FS

// GetStudioFS returns an http.FileSystem serving the embedded Quantum Log Studio UI.
func GetStudioFS() http.FileSystem {
	sub, err := fs.Sub(studioFS, "studio")
	if err != nil {
		return http.FS(studioFS)
	}
	return http.FS(sub)
}
