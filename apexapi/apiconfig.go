package apexapi

import (
	"os"
	"path/filepath"

	"github.com/tencent-connect/botgo/log"
	"gopkg.in/yaml.v3"
)

type API struct {
	ApiToken string
}

var ApiConf = API{}

func init() {
	if ApiConf.ApiToken != "" {
		log.Infof("API Token: %s", ApiConf.ApiToken)
		return
	}
	basePath, err := os.Getwd()
	if err != nil {
		log.Warnf("Failed to get current working directory: %v", err)
		return
	}

	confPath := filepath.Join(basePath, "conf", "config.yaml")
	log.Infof("Loading config from: %s", confPath)

	LoadConfig(confPath)
}

func LoadConfig(confPath string) {
	conf, err := os.ReadFile(confPath)
	if err != nil {
		log.Warnf("load config file failed, err:%v\n", err)
		return
	}
	err = yaml.Unmarshal(conf, &ApiConf)
	if err != nil {
		log.Warnf("parse config failed, err:%v\n", err)
		return
	}
}
