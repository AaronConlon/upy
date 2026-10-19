package analysis

import (
	"time"
)

// ProjectAnalysis 单个项目的分析详情
type ProjectAnalysis struct {
	Name            string     `json:"name"`
	Root            string     `json:"root"`
	Type            string     `json:"type"`                      // static | docker | system | -
	CurrentVersion  string     `json:"currentVersion"`
	PreviousVersion string     `json:"previousVersion,omitempty"`
	DeployedAt      *time.Time `json:"deployedAt,omitempty"`      // 解析后的时间戳
	DeployedAtRaw   string     `json:"deployedAtRaw,omitempty"`   // state.json 中的原始字符串
	Status          string     `json:"status"`                    // active | degraded | failed | undeployed | missing
	UnitName        string     `json:"unitName,omitempty"`

	// 空间统计 (字节)
	TotalBytes      int64 `json:"totalBytes"`
	ReleasesBytes   int64 `json:"releasesBytes"`
	ActiveBytes     int64 `json:"activeBytes"`     // 当前活跃版本
	OldReleaseBytes int64 `json:"oldReleaseBytes"` // 可清理旧版本
	BundlesBytes    int64 `json:"bundlesBytes"`    // 压缩包缓存
	BackupsBytes    int64 `json:"backupsBytes"`    // 归档备份
	SharedBytes     int64 `json:"sharedBytes"`     // shared 目录
	OtherBytes      int64 `json:"otherBytes"`      // 根目录直接文件或其余配置

	// 基准占比
	PercentOfMax   float64 `json:"percentOfMax"`   // 对标最大项目的百分比 (0.0 ~ 100.0)
	PercentOfTotal float64 `json:"percentOfTotal"` // 占全机 upy 空间的百分比

	// 状态标志
	IsMissingRoot bool `json:"isMissingRoot,omitempty"`
}

// AnalysisReport 全局分析汇总报表
type AnalysisReport struct {
	AnalyzedAt     time.Time         `json:"analyzedAt"`
	TotalProjects  int               `json:"totalProjects"`
	TotalBytes     int64             `json:"totalBytes"`
	MaxBytes       int64             `json:"maxBytes"`
	MaxProjectName string            `json:"maxProjectName"`
	LatestDeploy   *ProjectAnalysis  `json:"latestDeploy,omitempty"`
	Projects       []ProjectAnalysis `json:"projects"`
}
