package apexapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"image"
	_ "image/jpeg"

	"github.com/newton-miku/apexQQbot/apexapi"
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
	apexapi.StartLoadConfig(confPath)
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

func TestGenerateMapImage(t *testing.T) {
	// 切换到仓库根目录，加载配置
	os.Chdir("../")
	basePath, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	confPath := filepath.Join(basePath, "conf", "config.yaml")
	apexapi.StartLoadConfig(confPath)

	// 预检查：地图接口可达性
	if _, err := apexapi.GetMapRotate(); err != nil {
		t.Skipf("跳过：地图接口不可达或超时：%v", err)
	}

	// 生成地图图片
	path, err := apexapi.GenerateMapImage()
	if err != nil {
		t.Fatalf("GenerateMapImage 错误: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("生成的图片文件不存在: %s, err: %v", path, err)
	}
	absPath, _ := filepath.Abs(path)
	// 输出完整路径，便于手动检查
	t.Logf("生成的图片完整路径: %s", absPath)
	log.Infof("生成的图片完整路径: %s", absPath)

	// 解码图片并校验尺寸
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("图片解码失败: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 960 || b.Dy() != 900 {
		t.Fatalf("图片尺寸不符合预期，得到: %dx%d，期望: 960x900", b.Dx(), b.Dy())
	}
}

func TestGetMapResult(t *testing.T) {
	// 切换到仓库根目录，加载配置
	os.Chdir("../")
	basePath, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	confPath := filepath.Join(basePath, "conf", "config.yaml")
	apexapi.StartLoadConfig(confPath)

	// 调用集成结果函数
	path, err := apexapi.GetMapResult()
	if err != nil {
		t.Fatalf("GetMapResult 错误: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("生成的图片文件不存在: %s, err: %v", path, err)
	}
	absPath, _ := filepath.Abs(path)
	// 输出完整路径，便于手动检查
	t.Logf("生成的图片完整路径: %s", absPath)
	log.Infof("生成的图片完整路径: %s", absPath)

	// 解码图片并校验尺寸
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("图片解码失败: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 960 || b.Dy() != 900 {
		t.Fatalf("图片尺寸不符合预期，得到: %dx%d，期望: 960x900", b.Dx(), b.Dy())
	}
}
