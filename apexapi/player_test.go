package apexapi_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/newton-miku/apexQQbot/apexapi"
	"github.com/tencent-connect/botgo/log"
)

func TestGetPlayerData(t *testing.T) {
	os.Chdir("../")
	basePath, err := os.Getwd()
	if err != nil {
		log.Warnf("Failed to get current working directory: %v", err)
		return
	}

	confPath := filepath.Join(basePath, "conf", "config.yaml")
	apexapi.StartLoadConfig(confPath)

	res, err := apexapi.GetPlayerData(context.Background(), "Shdowmaker")
	if err != nil {
		t.Skipf("跳过：玩家接口不可达或超时：%v", err)
		return
	}
	log.Debugf("玩家名称: %s", res.Global.Name)
	log.Debugf("段位: %s %d", res.Global.Rank.RankName, res.Global.Rank.RankDiv)

	// 打印传奇信息，检查数据结构
	if selected, ok := res.Legends["selected"]; ok {
		log.Debugf("传奇名称: '%s'", selected.LegendName)
		log.Debugf("传奇数据项数: %d", len(selected.Data))
		for i, stat := range selected.Data {
			log.Debugf("  数据 %d: %s = %v", i, stat.Name, stat.Value)
		}
	} else {
		log.Debugf("未找到 selected 传奇")
	}

	// 打印原始 legends 结构
	log.Debugf("Legends 键: %v", func() []string {
		keys := make([]string, 0, len(res.Legends))
		for k := range res.Legends {
			keys = append(keys, k)
		}
		return keys
	}())

	// 打印原始 JSON 查看实际字段名
	raw, _ := json.MarshalIndent(res.Legends, "", "  ")
	log.Debugf("Legends 原始 JSON:\n%s", string(raw))
}
