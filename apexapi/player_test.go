package apexapi_test

import (
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

	res, err := apexapi.GetPlayerData("Shdowmaker")
	if err != nil {
		panic(err)
	}
	log.Debug(res)
}
