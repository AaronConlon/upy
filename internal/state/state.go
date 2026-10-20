// state.json 原子读写 (临时文件 + rename)
// 仅完整部署成功后才更新 currentVersion
package state

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AaronConlon/upy/internal/i18n"
)

// State 部署状态
type State struct {
	CurrentVersion  string `json:"currentVersion"`
	PreviousVersion string `json:"previousVersion"`
	DeployedAt      string `json:"deployedAt"`
	Type            string `json:"type,omitempty"`
	UnitName        string `json:"unitName,omitempty"`
	Status          string `json:"status,omitempty"` // active | degraded | failed
}

// Read 读取 state.json (不存在时返回空状态)
func Read(root string) (*State, error) {
	p := filepath.Join(root, "state.json")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, fmt.Errorf(i18n.T("无法读取 state.json: %v"), err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf(i18n.T("state.json 无效: %v"), err)
	}
	return &s, nil
}

// WriteAtomic 原子写入 state.json (临时文件 + rename)
func WriteAtomic(root string, s *State) error {
	p := filepath.Join(root, "state.json")
	tmp := p + ".tmp." + randomSuffix()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf(i18n.T("无法序列化 state.json: %v"), err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		os.Remove(tmp)
		return fmt.Errorf(i18n.T("无法写入 state.json: %v"), err)
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return fmt.Errorf(i18n.T("无法写入 state.json: %v"), err)
	}
	return nil
}

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
