// Deployer 工厂: 按 manifest 类型创建部署器
package deployer

import (
	"fmt"

	"github.com/AaronConlon/uply/internal/config"
)

// CreateDeployer 按 manifest.type + docker.mode 创建部署器
func CreateDeployer(ctx *DeployContext) (Deployer, error) {
	switch ctx.Manifest.Type {
	case "static":
		return NewStatic(ctx), nil
	case "docker":
		if ctx.Manifest.Docker.Mode == "compose" {
			return NewDockerCompose(ctx), nil
		}
		return NewDockerContainer(ctx), nil
	}
	return nil, fmt.Errorf("无法创建部署器: 未知类型 '%s'", ctx.Manifest.Type)
}

// HealthcheckTimeoutSec 取健康检查超时秒数
func HealthcheckTimeoutSec(m *config.Manifest) int {
	if m.Healthcheck == nil {
		return 30
	}
	switch t := m.Healthcheck.Timeout.(type) {
	case int:
		return t
	default:
		return 30
	}
}
