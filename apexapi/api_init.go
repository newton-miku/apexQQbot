package apexapi

import (
	botlog "github.com/tencent-connect/botgo/log"
)

func Init() {
	// 加载地图与模式翻译器（使用新的线程安全初始化）
	_ = getLegendsTranslator()
	_ = GetMapName("")       // 初始化地图翻译器
	_ = GetModeName("")      // 初始化模式翻译器

	// 初始化 SQLite 数据库
	if err := Players.Init(); err != nil {
		botlog.Errorf("初始化玩家数据库失败: %v", err)
	}

	// 自动从旧 JSON 文件迁移数据
	if err := Players.AutoMigrate(); err != nil {
		botlog.Warnf("迁移旧数据失败: %v，继续使用新数据库", err)
	}

	// 加载 API 配置
	LoadApexConfig()
}

// Close 关闭所有资源
func Close() {
	// 关闭数据库连接
	if err := Players.Close(); err != nil {
		botlog.Errorf("关闭数据库失败: %v\n", err)
	}
}

// LoadApexConfig 加载 API 配置（向后兼容）
func LoadApexConfig() {
	// 优先从配置文件加载
	if err := LoadConfig(); err != nil {
		botlog.Warnf("加载配置失败: %v，使用默认值\n", err)
	}
}
