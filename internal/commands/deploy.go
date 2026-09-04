// upy deploy [project_name]: 从本机项目注册表定位根目录并部署最新正式版本。
package commands

import (
	"fmt"
	"strings"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/ui"
)

// SelectProjectAndRelease 是无命令启动入口：选择项目后继续 release 的版本选择流程。
func SelectProjectAndRelease(force bool) error {
	project, ok, err := selectLocalProject("")
	if err != nil || !ok {
		return err
	}
	log.Info("已选择项目 " + project.Name + "（" + log.Path(project.Root) + "）")
	return Release(ReleaseArgs{Force: force, Root: project.Root})
}

// Deploy 按项目名（或交互选择）定位根目录，直接部署最新正式版本。
func Deploy(projectName string, force bool) error {
	project, ok, err := selectLocalProject(projectName)
	if err != nil || !ok {
		return err
	}
	log.Info("正在项目 " + project.Name + "（" + log.Path(project.Root) + "）部署最新版本。")
	// Root 贯穿下载、解压、部署与状态读写，效果等同于切入该目录执行 upy release latest。
	return Release(ReleaseArgs{Target: "latest", Force: force, Root: project.Root})
}

func selectLocalProject(projectName string) (config.LocalProject, bool, error) {
	projects, err := config.ListLocalProjects()
	if err != nil {
		return config.LocalProject{}, false, err
	}
	if len(projects) == 0 {
		log.Info("没有可用的本地项目。请先在项目根目录执行一次 upy release 成功部署。")
		return config.LocalProject{}, false, nil
	}

	projectName = strings.TrimSpace(projectName)
	if projectName != "" {
		for _, project := range projects {
			if project.Name == projectName {
				return project, true, nil
			}
		}
		return config.LocalProject{}, false, fmt.Errorf("找不到本地项目 %q。请先在项目根目录执行一次 upy release 成功部署", projectName)
	}

	options := make([]ui.SelectOption, len(projects))
	for i, project := range projects {
		options[i] = ui.SelectOption{Label: project.Name, Hint: project.Root}
	}
	idx, err := ui.Select("请选择本地项目", options)
	if err != nil {
		return config.LocalProject{}, false, err
	}
	return projects[idx], true, nil
}
