package server

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/http"
	"strings"
	"testing"
)

func TestPWAResourcesAndRemovedEnrollment(t *testing.T) {
	srv, mux, cookie, _ := newConsoleFixture(t)
	for _, auth := range []*http.Cookie{nil, cookie} {
		for _, path := range []string{"/auth", "/auth?format=json", "/auth?download=1"} {
			got := consoleRequest(mux, path, auth)
			if got.Code != 404 || strings.Contains(got.Body.String(), srv.totpMgr.Secret()) || strings.Contains(got.Body.String(), "otpauth:") {
				t.Fatalf("enrollment exposed via HTTP: %s", path)
			}
		}
	}
	resources := map[string]string{"/manifest.webmanifest": "application/manifest+json", "/sw.js": "text/javascript", "/pwa/offline.html": "text/html", "/pwa/icon.svg": "image/svg+xml", "/pwa/icon-192.png": "image/png", "/pwa/icon-512.png": "image/png", "/pwa/apple-touch-icon.png": "image/png"}
	for path, typ := range resources {
		r := consoleRequest(mux, path, nil)
		if r.Code != 200 || !strings.HasPrefix(r.Header().Get("Content-Type"), typ) {
			t.Fatalf("public resource %s: %d %s", path, r.Code, r.Header().Get("Content-Type"))
		}
		if strings.Contains(r.Body.String(), srv.totpMgr.Secret()) {
			t.Fatal("secret in public PWA asset")
		}
	}
	manifest := consoleRequest(mux, "/manifest.webmanifest", nil)
	var m struct {
		StartURL string `json:"start_url"`
		Display  string `json:"display"`
		Icons    []struct {
			Src string `json:"src"`
		}
	}
	if err := json.Unmarshal(manifest.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m.StartURL != "/overview" || m.Display != "standalone" {
		t.Fatal("incorrect application launch behavior")
	}
	for _, icon := range m.Icons {
		if consoleRequest(mux, icon.Src, nil).Code != 200 {
			t.Fatalf("missing manifest icon %s", icon.Src)
		}
	}
	for path, want := range map[string]int{"/pwa/icon-192.png": 192, "/pwa/icon-512.png": 512, "/pwa/apple-touch-icon.png": 180} {
		r := consoleRequest(mux, path, nil)
		cfg, err := png.DecodeConfig(bytes.NewReader(r.Body.Bytes()))
		if err != nil || cfg.Width != want || cfg.Height != want {
			t.Fatalf("invalid icon %s: %+v %v", path, cfg, err)
		}
	}
	sw := consoleRequest(mux, "/sw.js", nil)
	if sw.Header().Get("Service-Worker-Allowed") != "/" || sw.Header().Get("Cache-Control") != "no-cache" || strings.Contains(sw.Body.String(), "__UPY_CACHE__") {
		t.Fatal("worker scope/version headers incorrect")
	}
	login := consoleRequest(mux, "/", nil).Body.String()
	if !strings.Contains(login, `rel="manifest"`) || !strings.Contains(login, "upy server auth") || strings.Contains(login, "otpauth://") {
		t.Fatal("login page manifest/enrollment hint missing")
	}
}
