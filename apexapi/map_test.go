package apexapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/newton-miku/ApexQbot/apexapi"
	"github.com/tencent-connect/botgo/log"
)

func TestGetMapRotate(t *testing.T) {
	os.Chdir("../")
	basePath, err := os.Getwd()
	if err != nil {
		log.Warnf("Failed to get current working directory: %v", err)
		return
	}

	confPath := filepath.Join(basePath, "conf", "config.yaml")
	apexapi.LoadConfig(confPath)
	mapInfo, err := apexapi.GetMapRotate()
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := json.Marshal(mapInfo)
	if err != nil {
		t.Fatal(err)
	}
	log.Infof("MapInfo: %s", jsonData)
	time.Sleep(time.Second * 5)
	mapInfo, err = apexapi.GetMapRotate()
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err = json.Marshal(mapInfo)
	if err != nil {
		t.Fatal(err)
	}
	log.Infof("MapInfo: %s", jsonData)
}
