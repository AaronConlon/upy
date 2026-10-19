package bundle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// 构造一个含指定清单文件名的最小 zip
func writeNamedManifestZip(t *testing.T, filename, version string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundle.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	entry, err := w.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	manifest := "name: codia\nversion: " + version + "\ntype: docker\ndocker:\n  mode: compose\n"
	if _, err := entry.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestManifestVersionFromZip(t *testing.T) {
	t.Run("UpyYaml", func(t *testing.T) {
		path := writeNamedManifestZip(t, "upy.yaml", "1.0.0")
		got := ManifestVersionFromZip(path)
		if got != "1.0.0" {
			t.Fatalf("期望读到 1.0.0, 得到 %q", got)
		}
	})

	t.Run("DeployYamlCompat", func(t *testing.T) {
		path := writeNamedManifestZip(t, "deploy.yaml", "0.3.17")
		got := ManifestVersionFromZip(path)
		if got != "0.3.17" {
			t.Fatalf("期望读到 0.3.17, 得到 %q", got)
		}
	})
}

func TestManifestVersionFromZipMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if got := ManifestVersionFromZip(path); got != "" {
		t.Fatalf("缺少清单文件时应返回空串, 得到 %q", got)
	}
}
