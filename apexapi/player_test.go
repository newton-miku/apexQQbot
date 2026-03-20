package apexapi_test

import (
	"context"
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
}
