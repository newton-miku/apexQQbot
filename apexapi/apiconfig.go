package apexapi

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

type API struct {
	ApiToken string
}

var (
	ApiConf   = API{}
	config    Config
	configMx  sync.RWMutex
	configErr error
)

const defaultConfigFile = "conf/config.yaml"

// LoadConfig 加载配置（使用公共路径工具）
func LoadConfig() error {
	configMx.Lock()
	defer configMx.Unlock()

	// 优先使用公共路径工具
	configPath, err := GetConfigPath()
	if err != nil {
		configErr = fmt.Errorf("获取配置路径失败: %w", err)
		return configErr
	}

	return loadConfigFromFile(configPath)
}

// GetConfigPath 获取配置文件路径
func GetConfigPath() (string, error) {
	// 尝试可执行文件目录
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "conf", "config.yaml")
		if _, e := os.Stat(p); e == nil {
			return p, nil
		}
	}

	// 尝试资源目录
	assetDir, err := GetAssetPath()
	if err == nil {
		p := filepath.Join(assetDir, "..", "conf", "config.yaml")
		if _, e := os.Stat(p); e == nil {
			return p, nil
		}
	}

	// 回退到默认路径
	p := defaultConfigFile
	if _, e := os.Stat(p); e == nil {
		return p, nil
	}

	return "", fmt.Errorf("配置文件不存在: %s", defaultConfigFile)
}

// GetAPIConfig 获取 API 配置
func GetAPIConfig() API {
	configMx.RLock()
	defer configMx.RUnlock()
	return ApiConf
}

// GetAppConfig 获取应用配置
func GetAppConfig() Config {
	configMx.RLock()
	defer configMx.RUnlock()
	return config
}

func loadConfigFromFile(confPath string) error {
	conf, err := os.ReadFile(confPath)
	if err != nil {
		configErr = fmt.Errorf("读取配置文件失败: %w", err)
		return configErr
	}

	if err := yaml.Unmarshal(conf, &config); err != nil {
		configErr = fmt.Errorf("解析配置文件失败: %w", err)
		return configErr
	}

	if err := yaml.Unmarshal(conf, &ApiConf); err != nil {
		configErr = fmt.Errorf("解析API配置失败: %w", err)
		return configErr
	}

	return nil
}

// StartLoadConfig 启动时加载配置
func StartLoadConfig(confPath string) {
	configMx.Lock()
	defer configMx.Unlock()

	configErr = loadConfigFromFile(confPath)
}

// GetConfigError 获取配置加载错误
func GetConfigError() error {
	configMx.RLock()
	defer configMx.RUnlock()
	return configErr
}

func maskAppID(appID string) string {
	if len(appID) <= 4 {
		return "****"
	}
	return appID[:2] + "****" + appID[len(appID)-2:]
}
