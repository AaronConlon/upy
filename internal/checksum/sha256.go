// Package checksum 校验 Release 资产完整性。
package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifySHA256SUMS 用 SHA256SUMS 中指定资产的摘要校验本地文件。
// 清单使用 GNU coreutils 标准格式：<hex>  <filename> 或 <hex> *<filename>。
func VerifySHA256SUMS(filePath, sumsPath, assetName string) error {
	data, err := os.ReadFile(sumsPath)
	if err != nil {
		return fmt.Errorf("无法读取 SHA256SUMS: %w", err)
	}
	expected, err := expectedSHA256(string(data), assetName)
	if err != nil {
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("无法读取下载资产: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("计算 SHA-256 失败: %w", err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("SHA-256 校验失败：资产 %s 的摘要与 Release 清单不一致", assetName)
	}
	return nil
}

func expectedSHA256(sums, assetName string) (string, error) {
	var matches []string
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name != assetName {
			continue
		}
		digest := strings.ToLower(fields[0])
		if len(digest) != 64 {
			return "", fmt.Errorf("SHA256SUMS 中 %s 的摘要格式无效", assetName)
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return "", fmt.Errorf("SHA256SUMS 中 %s 的摘要格式无效", assetName)
		}
		matches = append(matches, digest)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("SHA256SUMS 中不存在资产 %s", assetName)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("SHA256SUMS 中存在重复资产 %s", assetName)
	}
	return matches[0], nil
}
