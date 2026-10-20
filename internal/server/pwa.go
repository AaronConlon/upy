package server

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/AaronConlon/upy/internal/version"
)

// PWA 资源全部公开且不含账户信息。动态控制台继续由 requireAuth 保护。
func (s *Server) registerPWARoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		servePWAAsset(w, r, "manifest.webmanifest", "application/manifest+json")
	})
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		data, _ := consoleAssets.ReadFile("web/pwa/sw.js")
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Service-Worker-Allowed", "/")
		fmt.Fprint(w, strings.ReplaceAll(string(data), "__UPY_CACHE__", pwaCacheName()))
	})
	for name, contentType := range map[string]string{"offline.html": "text/html; charset=utf-8", "icon.svg": "image/svg+xml", "icon-192.png": "image/png", "icon-512.png": "image/png", "apple-touch-icon.png": "image/png"} {
		mux.HandleFunc("GET /pwa/"+name, func(w http.ResponseWriter, r *http.Request) { servePWAAsset(w, r, name, contentType) })
	}
}
func servePWAAsset(w http.ResponseWriter, r *http.Request, name, contentType string) {
	data, err := consoleAssets.ReadFile("web/pwa/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}
func pwaCacheName() string {
	hash := sha256.New()
	hash.Write([]byte(version.String()))
	for _, name := range []string{"console.html", "styles.css", "tokens.css", "app.js"} {
		data, _ := consoleAssets.ReadFile("web/" + name)
		hash.Write(data)
	}
	for _, name := range []string{"sw.js", "offline.html", "icon.svg", "icon-192.png", "icon-512.png", "apple-touch-icon.png"} {
		data, _ := consoleAssets.ReadFile("web/pwa/" + name)
		hash.Write(data)
	}
	return fmt.Sprintf("upy-pwa-%x", hash.Sum(nil)[:12])
}
