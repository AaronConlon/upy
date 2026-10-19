package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/state"
	"gopkg.in/yaml.v3"
)

// CollectAll 收集所有纳管项目的信息（包含可选的当前目录项目）
func CollectAll(candidateRoot string) ([]ProjectAnalysis, error) {
	localProjects, err := config.ListLocalProjects()
	if err != nil {
		localProjects = nil
	}

	seenRoots := make(map[string]bool, len(localProjects)+1)
	seenNames := make(map[string]bool, len(localProjects)+1)

	var list []ProjectAnalysis

	for _, lp := range localProjects {
		absRoot, err := filepath.Abs(lp.Root)
		if err != nil {
			absRoot = lp.Root
		}
		seenRoots[absRoot] = true
		seenNames[lp.Name] = true

		pa := CollectProject(lp.Name, absRoot)
		list = append(list, pa)
	}

	// 如果 candidateRoot 是有效的项目目录且未在登记列表中，自动加入分析范围
	if candidateRoot != "" {
		absCandidate, err := filepath.Abs(candidateRoot)
		if err == nil && !seenRoots[absCandidate] {
			if isProjectDir(absCandidate) {
				name := detectProjectName(absCandidate)
				if name == "" {
					name = filepath.Base(absCandidate)
				}
				pa := CollectProject(name, absCandidate)
				list = append(list, pa)
			}
		}
	}

	return list, nil
}

// CollectProject 深度收集单个项目的元数据与磁盘占用
func CollectProject(name, root string) ProjectAnalysis {
	pa := ProjectAnalysis{
		Name: name,
		Root: root,
		Type: "-",
	}

	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		pa.IsMissingRoot = true
		pa.Status = "missing"
		return pa
	}

	// 1. 读取 state.json
	st, err := state.Read(root)
	if err == nil && st != nil {
		pa.CurrentVersion = st.CurrentVersion
		pa.PreviousVersion = st.PreviousVersion
		pa.DeployedAtRaw = st.DeployedAt
		pa.UnitName = st.UnitName
		pa.Status = st.Status
		if st.Type != "" {
			pa.Type = normalizeType(st.Type)
		}
		if st.DeployedAt != "" {
			if t, err := time.Parse(time.RFC3339, st.DeployedAt); err == nil {
				pa.DeployedAt = &t
			} else if t, err := time.Parse(time.RFC3339Nano, st.DeployedAt); err == nil {
				pa.DeployedAt = &t
			}
		}
	}

	// 2. 解析当前活跃版本 (若 state 未记录则尝试解析 current 软链)
	if pa.CurrentVersion == "" {
		if link, err := os.Readlink(filepath.Join(root, "current")); err == nil {
			pa.CurrentVersion = filepath.Base(link)
		}
	}

	// 3. 探测部署类型 (如果 state 中未记录)
	if pa.Type == "-" || pa.Type == "" {
		if pa.CurrentVersion != "" {
			pa.Type = detectTypeFromDir(filepath.Join(root, "releases", pa.CurrentVersion))
		}
		if pa.Type == "-" || pa.Type == "" {
			pa.Type = detectTypeFromDir(root)
		}
		if pa.Type == "" {
			pa.Type = "-"
		}
	}

	// 4. 纠正运行状态
	if pa.Status == "" {
		if pa.CurrentVersion != "" {
			pa.Status = "active"
		} else {
			pa.Status = "undeployed"
		}
	}

	// 5. 递归测算各子模块空间占用（严格不穿透软链接）
	measureProjectDisk(&pa)

	return pa
}

// measureProjectDisk 递归测算项目空间，按 releases/bundles/backups/shared/other 细分
func measureProjectDisk(pa *ProjectAnalysis) {
	entries, err := os.ReadDir(pa.Root)
	if err != nil {
		return
	}

	for _, entry := range entries {
		entryName := entry.Name()
		entryPath := filepath.Join(pa.Root, entryName)

		switch entryName {
		case "releases":
			measureReleases(entryPath, pa)
		case "bundles":
			pa.BundlesBytes = calculateSize(entryPath)
		case "backups":
			pa.BackupsBytes = calculateSize(entryPath)
		case "shared":
			pa.SharedBytes = calculateSize(entryPath)
		default:
			// 其它文件、目录或根级软链 (如 current -> releases/vX.Y.Z)
			pa.OtherBytes += calculateSize(entryPath)
		}
	}

	pa.TotalBytes = pa.ReleasesBytes + pa.BundlesBytes + pa.BackupsBytes + pa.SharedBytes + pa.OtherBytes
}

// measureReleases 细分 releases 目录：区分当前活跃版本与历史旧版本
func measureReleases(releasesDir string, pa *ProjectAnalysis) {
	subEntries, err := os.ReadDir(releasesDir)
	if err != nil {
		return
	}

	for _, sub := range subEntries {
		subPath := filepath.Join(releasesDir, sub.Name())
		subSize := calculateSize(subPath)

		if sub.Name() == pa.CurrentVersion && pa.CurrentVersion != "" {
			pa.ActiveBytes += subSize
		} else {
			pa.OldReleaseBytes += subSize
		}
	}

	pa.ReleasesBytes = pa.ActiveBytes + pa.OldReleaseBytes
}

// calculateSize 递归计算目录或文件字节大小。严禁递归穿透软链接，杜绝重复计算。
func calculateSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		// 遇到符号链接：仅记录该符号链接节点本身大小（~数十字节），不递归深入目标文件/目录
		if d.Type()&os.ModeSymlink != 0 {
			total += info.Size()
			return nil
		}
		if !d.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func isProjectDir(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "deploy.yaml"))
	return err == nil && !info.IsDir()
}

func detectProjectName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "deploy.yaml"))
	if err != nil {
		return ""
	}
	var probe struct {
		Name string `yaml:"name"`
	}
	if err := yaml.Unmarshal(data, &probe); err == nil {
		return strings.TrimSpace(probe.Name)
	}
	return ""
}

func detectTypeFromDir(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "deploy.yaml"))
	if err != nil {
		return ""
	}
	var probe struct {
		Type    string `yaml:"type"`
		Service any    `yaml:"service"`
		System  any    `yaml:"system"`
		Docker  any    `yaml:"docker"`
		Static  any    `yaml:"static"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return ""
	}
	if probe.Type != "" {
		return normalizeType(probe.Type)
	}
	if probe.Service != nil || probe.System != nil {
		return "system"
	}
	if probe.Docker != nil {
		return "docker"
	}
	if probe.Static != nil {
		return "static"
	}
	return ""
}

func normalizeType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "system", "service":
		return "system"
	case "docker", "compose":
		return "docker"
	case "static", "spa":
		return "static"
	default:
		return t
	}
}
