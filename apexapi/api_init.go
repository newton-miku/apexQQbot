package apexapi

import (
	"time"

	"github.com/tencent-connect/botgo/log"
)

func Init() {
	//  加载地图配置
	LoadModeDict(modeDictPath)
	StartMapDictReloader(10*time.Second, mapDictPath)

	// 加载玩家绑定数据
	err := Players.loadPlayerData()
	if err != nil {
		log.Errorf("load player data failed, err:%v\n", err)
	}

	// 加载API配置
	LoadApexConfig()
}
