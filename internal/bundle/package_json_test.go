package bundle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestReadPackageJSONVersion(t *testing.T) {
	tmp := t.TempDir()

	// 1. 无 package.json
	if v := ReadPackageJSONVersion(tmp); v != "" {
		t.Fatalf("期望空字符串，得到: %q", v)
	}

	// 2. 有有效 package.json
	content := `{"name": "test-app", "version": "1.2.3"}`
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := ReadPackageJSONVersion(tmp); v != "1.2.3" {
		t.Fatalf("期望 1.2.3，得到: %q", v)
	}
}

func TestReadPackageJSONVersionFromZip(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "test.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("package.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`{"name": "demo", "version": "2.0.4"}`)); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	f.Close()

	if v := ReadPackageJSONVersionFromZip(zipPath); v != "2.0.4" {
		t.Fatalf("期望 2.0.4，得到: %q", v)
	}
}
