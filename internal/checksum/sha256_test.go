package checksum

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySHA256SUMS(t *testing.T) {
	dir := t.TempDir()
	asset := filepath.Join(dir, "upy-linux-x64")
	sums := filepath.Join(dir, "SHA256SUMS")
	if err := os.WriteFile(asset, []byte("verified binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	// sha256("verified binary")
	const digest = "86fd6fb55a10988213329d914da3f5fbbc213ee143b46148ed21b60d9454e3dc"
	if err := os.WriteFile(sums, []byte(digest+"  upy-linux-x64\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256SUMS(asset, sums, "upy-linux-x64"); err != nil {
		t.Fatal(err)
	}
}

func TestVerifySHA256SUMSRejectsMismatchAndMissingAsset(t *testing.T) {
	dir := t.TempDir()
	asset := filepath.Join(dir, "upy-linux-x64")
	sums := filepath.Join(dir, "SHA256SUMS")
	if err := os.WriteFile(asset, []byte("unexpected binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sums, []byte("86fd6fb55a10988213329d914da3f5fbbc213ee143b46148ed21b60d9454e3dc  upy-linux-x64\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256SUMS(asset, sums, "upy-linux-x64"); err == nil {
		t.Fatal("摘要不匹配时应拒绝")
	}
	if err := VerifySHA256SUMS(asset, sums, "upy-darwin-arm64"); err == nil {
		t.Fatal("资产不在清单中时应拒绝")
	}
}
