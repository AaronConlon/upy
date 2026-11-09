// upy deploy [project_name]: 从本机项目注册表定位根目录并部署最新正式版本。
package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/state"
	"github.com/AaronConlon/upy/internal/ui"
)

// SelectProjectAndRelease 是无命令启动入口：选择项目后继续 release 的版本选择流程。
func SelectProjectAndRelease(force, verbose bool, startedAt ...time.Time) error {
	var start time.Time
	if len(startedAt) > 0 && !startedAt[0].IsZero() {
		start = startedAt[0]
	} else {
		start = time.Now()
	}
	project, ok, err := selectLocalProject("")
	if err != nil || !ok {
		return err
	}
	log.Info(i18n.T("已选择项目 ") + project.Name + "（" + log.Path(project.Root) + "）")
	return Release(ReleaseArgs{Force: force, Verbose: verbose, Root: project.Root, StartedAt: start})
}

// Deploy 按项目名（或交互选择）定位根目录，直接部署最新正式版本。
func Deploy(projectName string, force, verbose bool, startedAt ...time.Time) error {
	var start time.Time
	if len(startedAt) > 0 && !startedAt[0].IsZero() {
		start = startedAt[0]
	} else {
		start = time.Now()
	}
	project, ok, err := selectLocalProject(projectName)
	if err != nil || !ok {
		return err
	}
	log.Info(i18n.T("正在项目 ") + project.Name + "（" + log.Path(project.Root) + i18n.T("）部署最新版本。"))
	// Root 贯穿下载、解压、部署与状态读写，效果等同于切入该目录执行 upy release latest。
	return Release(ReleaseArgs{Target: "latest", Force: force, Verbose: verbose, Root: project.Root, StartedAt: start})
}

func selectLocalProject(projectName string) (config.LocalProject, bool, error) {
	projects, err := config.ListLocalProjects()
	if err != nil {
		return config.LocalProject{}, false, err
	}
	if len(projects) == 0 {
		log.Info(i18n.T("没有可用的本地项目。请先在项目根目录执行一次 upy release 成功部署。"))
		return config.LocalProject{}, false, nil
	}

	projectName = strings.TrimSpace(projectName)
	if projectName != "" {
		for _, project := range projects {
			if project.Name == projectName {
				return project, true, nil
			}
		}
		return config.LocalProject{}, false, fmt.Errorf(i18n.T("找不到本地项目 %q。请先在项目根目录执行一次 upy release 成功部署"), projectName)
	}

	options := make([]ui.SelectOption, len(projects))
	for i, project := range projects {
		label := project.Name
		badge := ""
		warn := ""
		hint := project.Root

		if _, err := os.Stat(project.Root); err != nil {
			warn = i18n.T("⚠️ 目录不可访问")
		}

		if st, err := state.Read(project.Root); err == nil && st != nil && st.CurrentVersion != "" {
			label = fmt.Sprintf("%s [%s · %s]", project.Name, st.CurrentVersion, st.Type)
			badge = i18n.T("(运行中 / ACTIVE)")
			if st.DeployedAt != "" {
				hint += " · " + i18n.T("最近部署: ") + st.DeployedAt
			}
		} else {
			badge = i18n.T("(未部署)")
		}

		options[i] = ui.SelectOption{
			Label: label,
			Hint:  hint,
			Badge: badge,
			Warn:  warn,
		}
	}
	idx, err := ui.Select(i18n.T("请选择本地项目"), options)
	if err != nil {
		return config.LocalProject{}, false, err
	}
	return projects[idx], true, nil
}
