package analysis

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Analyze 对采集到的各项目执行基准计算、百分比换算、全机最新部署甄选及排序
func Analyze(projects []ProjectAnalysis, sortBy string) *AnalysisReport {
	report := &AnalysisReport{
		AnalyzedAt:    time.Now(),
		TotalProjects: len(projects),
		Projects:      make([]ProjectAnalysis, len(projects)),
	}
	copy(report.Projects, projects)

	if len(report.Projects) == 0 {
		return report
	}

	// 1. 统计全机累计空间、寻找占用最高项目 (100% 基准) 与全机最新部署
	var totalBytes int64
	var maxBytes int64
	var maxProject string
	var latestProject *ProjectAnalysis
	var latestTime time.Time

	for i := range report.Projects {
		p := &report.Projects[i]
		totalBytes += p.TotalBytes

		if p.TotalBytes > maxBytes {
			maxBytes = p.TotalBytes
			maxProject = p.Name
		}

		if p.DeployedAt != nil {
			if latestProject == nil || p.DeployedAt.After(latestTime) {
				latestTime = *p.DeployedAt
				latestProject = p
			}
		}
	}

	report.TotalBytes = totalBytes
	report.MaxBytes = maxBytes
	report.MaxProjectName = maxProject

	// 2. 计算各项目的对标百分比
	for i := range report.Projects {
		p := &report.Projects[i]
		if maxBytes > 0 {
			p.PercentOfMax = math.Round((float64(p.TotalBytes)/float64(maxBytes)*100.0)*10) / 10
		} else {
			p.PercentOfMax = 0.0
		}

		if totalBytes > 0 {
			p.PercentOfTotal = math.Round((float64(p.TotalBytes)/float64(totalBytes)*100.0)*10) / 10
		} else {
			p.PercentOfTotal = 0.0
		}
	}

	if latestProject != nil {
		copyLatest := *latestProject
		report.LatestDeploy = &copyLatest
	}

	// 3. 执行排序
	sortProjects(report.Projects, sortBy)

	return report
}

// sortProjects 根据指定的策略排序
func sortProjects(projects []ProjectAnalysis, sortBy string) {
	sortBy = strings.ToLower(strings.TrimSpace(sortBy))
	switch sortBy {
	case "time":
		sort.Slice(projects, func(i, j int) bool {
			pi, pj := projects[i], projects[j]
			if pi.DeployedAt != nil && pj.DeployedAt != nil {
				if !pi.DeployedAt.Equal(*pj.DeployedAt) {
					return pi.DeployedAt.After(*pj.DeployedAt)
				}
			} else if pi.DeployedAt != nil {
				return true
			} else if pj.DeployedAt != nil {
				return false
			}
			return pi.Name < pj.Name
		})
	case "name":
		sort.Slice(projects, func(i, j int) bool {
			return projects[i].Name < projects[j].Name
		})
	case "size":
		fallthrough
	default:
		// 默认按空间占用降序排列 (首行即为 100% 基准大户)
		sort.Slice(projects, func(i, j int) bool {
			if projects[i].TotalBytes != projects[j].TotalBytes {
				return projects[i].TotalBytes > projects[j].TotalBytes
			}
			return projects[i].Name < projects[j].Name
		})
	}
}

// FormatRelativeTime 格式化相对时间 (如 "刚刚"、"22分钟前"、"3天前")
func FormatRelativeTime(t *time.Time, now time.Time) string {
	if t == nil {
		return "-"
	}
	diff := now.Sub(*t)
	if diff < 0 {
		return "刚刚"
	}
	if diff < time.Minute {
		return "刚刚"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins <= 0 {
			mins = 1
		}
		return fmt.Sprintf("%d分钟前", mins)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours <= 0 {
			hours = 1
		}
		return fmt.Sprintf("%d小时前", hours)
	}
	if diff < 30*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days <= 0 {
			days = 1
		}
		return fmt.Sprintf("%d天前", days)
	}
	return t.Format("2006-01-02")
}
