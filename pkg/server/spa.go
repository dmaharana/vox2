package server

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

// SPAHandler serves a single page application from an fs.FS filesystem, falling back to index.html for client-side routes.
type SPAHandler struct {
	fileSystem http.FileSystem
	indexHTML  []byte
}

// NewSPAHandler creates a new SPA handler.
func NewSPAHandler(fileSys fs.FS) *SPAHandler {
	httpFS := http.FS(fileSys)
	indexBytes, _ := fs.ReadFile(fileSys, "index.html")
	return &SPAHandler{
		fileSystem: httpFS,
		indexHTML:  indexBytes,
	}
}

func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")

	// If root or empty, serve index.html
	if path == "" || path == "." {
		h.serveIndex(w)
		return
	}

	// Check if file exists in filesystem
	f, err := h.fileSystem.Open(path)
	if err != nil {
		// Fallback to index.html for SPA routes
		h.serveIndex(w)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		h.serveIndex(w)
		return
	}

	http.FileServer(h.fileSystem).ServeHTTP(w, r)
}

func (h *SPAHandler) serveIndex(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(h.indexHTML) > 0 {
		_, _ = w.Write(h.indexHTML)
	} else {
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Agent Harness</title></head><body><div id="root">Agent Harness UI</div></body></html>`))
	}
}
