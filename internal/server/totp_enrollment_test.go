package server

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTOTPLegacyEnrollmentRemainsValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "totp.json")
	original := []byte(`{"secret":"JBSWY3DPEHPK3PXP","views":5,"created_at":"2026-01-01T00:00:00Z"}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	tm, err := NewTOTPManager(path, "")
	if err != nil {
		t.Fatal(err)
	}
	code, err := generateTOTP("JBSWY3DPEHPK3PXP", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !tm.Validate(code) {
		t.Fatal("legacy authenticator stopped working")
	}
	uri, err := url.Parse(tm.ProvisioningURI("admin@vps", "upy"))
	if err != nil {
		t.Fatal(err)
	}
	if uri.Scheme != "otpauth" || uri.Query().Get("secret") != "JBSWY3DPEHPK3PXP" || uri.Query().Get("digits") != "6" {
		t.Fatalf("unexpected enrollment URI: %s", uri)
	}
	qr, err := tm.TerminalQRCode("admin@vps")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(qr, "█") || strings.Contains(qr, tm.Secret()) || strings.Contains(qr, "otpauth://") {
		t.Fatal("expected QR only, without plaintext secret")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatal("enrollment must not modify existing secret file")
	}
}
func TestTOTPInvalidFileNotReplaced(t *testing.T) {
	for _, body := range []string{"broken", `{"secret":"invalid!"}`, `{"secret":""}`} {
		path := filepath.Join(t.TempDir(), "totp.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewTOTPManager(path, ""); err == nil {
			t.Fatal("invalid file accepted")
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != body {
			t.Fatal("invalid secret was silently replaced")
		}
	}
}
func TestTOTPConcurrentInitialization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "totp.json")
	var wg sync.WaitGroup
	results := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tm, err := NewTOTPManager(path, "")
			if err != nil {
				t.Error(err)
				return
			}
			results <- tm.Secret()
		}()
	}
	wg.Wait()
	close(results)
	secret := ""
	for value := range results {
		if secret == "" {
			secret = value
		}
		if secret != value {
			t.Fatal("concurrent enrollment created different secrets")
		}
	}
	if secret == "" {
		t.Fatal("no initialized secret")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("secret file permissions: %o", info.Mode().Perm())
	}
}
