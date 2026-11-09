package bundle

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type packageJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ReadPackageJSONVersion 从指定目录读取 package.json 中的 version 字段 (不存在或无 version 返回空字符串)
func ReadPackageJSONVersion(dir string) string {
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return ""
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return strings.TrimSpace(pkg.Version)
}

// ReadPackageJSONVersionFromZip 从 zip 压缩包中读取 package.json 的 version 字段
func ReadPackageJSONVersionFromZip(zipPath string) string {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return ""
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "package.json" || strings.HasSuffix(f.Name, "/package.json") {
			// 优先取根目录 package.json
			if f.Name == "package.json" {
				return readVersionFromZipEntry(f)
			}
		}
	}
	return ""
}

func readVersionFromZipEntry(f *zip.File) string {
	rc, err := f.Open()
	if err != nil {
		return ""
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return ""
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return strings.TrimSpace(pkg.Version)
}
