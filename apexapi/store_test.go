package apexapi_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/newton-miku/apexQQbot/apexapi"
)

func TestGetStoreCountdown(t *testing.T) {
	os.Chdir("../")
	basePath, err := os.Getwd()
	if err != nil {
		t.Skipf("无法获取工作目录: %v", err)
		return
	}

	confPath := filepath.Join(basePath, "conf", "config.yaml")
	apexapi.StartLoadConfig(confPath)

	countdown, err := apexapi.GetStoreCountdown()
	if err != nil {
		t.Logf("获取商店倒计时失败: %v", err)
		// API 可能暂时不可用，不视为测试失败
		t.Skip("API 不可达")
		return
	}

	t.Logf("截止时间: %v", countdown.Deadline)
	t.Logf("剩余时间: %s", countdown.String())
	t.Logf("是否有效: %v", countdown.IsValid())
}

// go test -v .\apexapi\ -run TestGetStoreCountdown
