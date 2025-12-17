package apexapi

import (
	"github.com/newton-miku/apexQQbot/tools"
	"github.com/tencent-connect/botgo/log"
)

func Init() {
	// 加载地图与模式翻译器（热更新）
	if t, err := tools.NewTranslator(modeDictPath); err == nil {
		modeTranslator = t
	}
	if t, err := tools.NewTranslator(mapDictPath); err == nil {
		mapTranslator = t
	}
	// 加载传奇翻译器
	if t, err := tools.NewTranslator(legendsDictPath); err == nil {
		legendsTranslator = t
	}

	// 加载玩家绑定数据
	err := Players.loadPlayerData()
	if err != nil {
		log.Errorf("load player data failed, err:%v\n", err)
	}

	// 加载API配置
	LoadApexConfig()
	log.Debug("api config loaded\n")
}
