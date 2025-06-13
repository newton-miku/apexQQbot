package apexapi

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/newton-miku/ApexQbot/tools"
	botlog "github.com/tencent-connect/botgo/log"
	"golang.org/x/image/draw"
)

// 自定义时间类型以支持从整数解析
type UnixTime time.Time

func (t *UnixTime) UnmarshalJSON(data []byte) error {
	var timestamp int64
	if err := json.Unmarshal(data, &timestamp); err != nil {
		return err
	}
	*t = UnixTime(time.Unix(timestamp, 0))
	return nil
}
func (t UnixTime) MarshalJSON() ([]byte, error) {
	timestamp := time.Time(t).Unix()
	return json.Marshal(timestamp)
}

// 单张地图的信息
type MapInfo struct {
	Name      string   `json:"map"`
	Code      string   `json:"code"`
	StartTime UnixTime `json:"start"`
	EndTime   UnixTime `json:"end"`
	//  素材URL
	Asset string
}

// 当前模式下的地图信息
type MapRotateInfo struct {
	Current MapInfo `json:"current"`
	Next    MapInfo `json:"next"`
}

func getMapInfo(mapRotate MapRotateInfo) []MapInfo {
	return []MapInfo{
		mapRotate.Current,
		mapRotate.Next,
	}
}

// 所有地图的轮换信息
type MapRotate struct {
	Battle_royale MapRotateInfo `json:"battle_royale"`
	Ranked        MapRotateInfo `json:"ranked"`
	Ltm           MapRotateInfo `json:"ltm"`
}

func getMapRotate(mapRotate MapRotate) []MapRotateInfo {
	return []MapRotateInfo{
		mapRotate.Battle_royale,
		mapRotate.Ranked,
		mapRotate.Ltm,
	}
}

var (
	cachedMapRotate MapRotate
	cacheExpiresAt  time.Time
)

func init() {
	LoadModeDict(modeDictPath)
	StartMapDictReloader(10*time.Second, mapDictPath)
}

// 获取所有 Current Map 中最早的 EndTime
func getEarliestEndTime(mr MapRotate) time.Time {
	times := []time.Time{
		time.Time(mr.Battle_royale.Current.EndTime),
		time.Time(mr.Ranked.Current.EndTime),
		time.Time(mr.Ltm.Current.EndTime),
	}

	min := times[0]
	for _, t := range times[1:] {
		if t.Before(min) {
			min = t
		}
	}

	return min
}
func GetMapRotate() (MapRotate, error) {
	// 如果缓存有效，直接返回
	if !cacheExpiresAt.IsZero() && time.Now().Before(cacheExpiresAt) {
		remaining := time.Until(cacheExpiresAt)
		botlog.Debugf("使用地图缓存，剩余缓存时间：%d 秒 (%s)", int64(remaining.Seconds()), remaining.String())
		return cachedMapRotate, nil
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("GET", "https://api.mozambiquehe.re/maprotation?version=2", nil)
	if err != nil {
		return MapRotate{}, fmt.Errorf("%w: %v", ErrRequestCreateFailed, err)
	}
	req.Header.Set("Authorization", ApiConf.ApiToken)

	resp, err := client.Do(req)
	if err != nil {
		return MapRotate{}, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()
	limitReader := io.LimitReader(resp.Body, 5<<10) // 最大 5KB
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return MapRotate{}, ErrReadResponseFailed
	}
	slog.Debug("获取地图轮换", "statusCode", resp.StatusCode)
	if resp.StatusCode == http.StatusOK {
		var mapRotate MapRotate
		err := json.Unmarshal(body, &mapRotate)
		if err != nil {
			botlog.Debugf("JSON解析错误: %v", err)
			return MapRotate{}, ErrInvalidJSON
		}

		// 更新缓存和过期时间
		cachedMapRotate = mapRotate
		cacheExpiresAt = getEarliestEndTime(mapRotate)

		return mapRotate, nil
	}
	return MapRotate{}, ErrStatusCode(resp.StatusCode)
}

var (
	MapDict       map[string]string
	mapDictMutex  sync.RWMutex
	ModeDict      map[string]string
	modeDictMutex sync.RWMutex
	mapDictPath   = "./asset/map_dict.json"
	modeDictPath  = "./asset/mode_dict.json"
	assetDir      = "./asset/"
	assetFontDir  = "./asset/Font"
	assetCacheDir = "./asset/Map"
	lastModTime   time.Time
)

// LoadMapDict 只在文件修改后重新加载
func LoadMapDict(path string) (int, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 2, err
	}

	// 如果文件未修改，则跳过加载
	if !fileInfo.ModTime().After(lastModTime) {
		// log.Debug("地图字典文件未变化，跳过重载")
		return 1, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return 2, err
	}

	var dict map[string]string
	if err := json.Unmarshal(data, &dict); err != nil {
		return 2, err
	}

	mapDictMutex.Lock()
	defer mapDictMutex.Unlock()
	MapDict = dict
	lastModTime = fileInfo.ModTime() // 更新最后修改时间
	botlog.Debug("地图字典已重载", MapDict)
	log.Print("地图字典已重载")

	return 0, nil
}

// LoadModeDict
func LoadModeDict(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 2, err
	}

	var dict map[string]string
	if err := json.Unmarshal(data, &dict); err != nil {
		return 2, err
	}

	modeDictMutex.Lock()
	defer modeDictMutex.Unlock()
	ModeDict = dict

	return 0, nil
}

// 获取地图中文名（线程安全）
func GetMapName(code string) string {
	mapDictMutex.RLock()
	defer mapDictMutex.RUnlock()

	if name, ok := MapDict[code]; ok {
		return name
	}
	return code // 缺失返回code
}

// 获取模式中文名（线程安全）
func GetModeName(code string) string {
	modeDictMutex.RLock()
	defer modeDictMutex.RUnlock()

	if name, ok := ModeDict[code]; ok {
		return name
	}
	return code // 缺失返回code
}

// 加载地图字典
func StartMapDictReloader(interval time.Duration, path string) {
	go func() {
		for {
			if code, err := LoadMapDict(path); err != nil {
				botlog.Errorf("重新加载地图字典失败: %v", err)
			} else {
				switch code { // 0: 已重载字典, 1: 地图字典文件无变化, 2: 无效的JSON格式
				case 0:
					botlog.Info("地图字典已重载")
				case 1:
					// log.Debug("地图字典文件无变化")
				case 2:
					botlog.Error("地图字典重载错误", err)
				default:
				}
			}
			time.Sleep(interval)
		}
	}()
}

// 下载并缓存图片
func CacheImage(urlLink string) (string, error) {
	// 解析URL获取文件名
	u, err := url.Parse(urlLink)
	if err != nil {
		return "", fmt.Errorf("解析URL失败: %v", err)
	}
	filename := filepath.Base(u.Path)

	// 创建缓存路径
	cachePath := filepath.Join(assetCacheDir, filename)

	// 确保缓存目录存在 ✅ 新增部分
	if err := os.MkdirAll(assetCacheDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("创建缓存目录失败: %v", err)
	}

	// 检查是否已有缓存
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	// 下载图片
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("GET", urlLink, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP错误: %d", resp.StatusCode)
	}

	// 创建并写入文件（带 O_EXCL 防止竞态）
	outFile, err := os.OpenFile(cachePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %v", err)
	}
	defer func() {
		_ = outFile.Close()
		if err != nil {
			// 出现错误时删除残留文件
			_ = os.Remove(cachePath)
		}
	}()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("写入文件失败: %v", err)
	}

	// 再次检查路径是否在 assetCacheDir 内部（防止路径穿越）
	absCachePath, err := filepath.Abs(cachePath)
	if err != nil {
		return "", fmt.Errorf("获取绝对路径失败: %v", err)
	}
	absAssetCacheDir, err := filepath.Abs(assetCacheDir)
	if err != nil {
		return "", fmt.Errorf("获取缓存目录绝对路径失败: %v", err)
	}
	if !strings.HasPrefix(absCachePath, absAssetCacheDir+string(filepath.Separator)) {
		return "", fmt.Errorf("文件路径非法: %s", absCachePath)
	}

	return cachePath, nil
}

// 格式化持续时间为 "HH:mm:ss"
func formatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// GenerateMapImage 生成地图轮换信息图片
func GenerateMapImage() (string, error) {
	mapRotate, err := GetMapRotate()
	if err != nil {
		return "", fmt.Errorf("获取地图轮换信息失败：%v", err)
	}

	selMap := []string{"battle_royale", "ranked", "ltm"}
	var images []image.Image

	for i, modeMap := range getMapRotate(mapRotate) {
		current := modeMap.Current
		next := modeMap.Next

		imgPath, err := CacheImage(current.Asset)
		if err != nil {
			return "", fmt.Errorf("缓存图片失败：%v", err)
		}
		// 调整图片大小
		imgFile, err := os.Open(imgPath)
		if err != nil {
			return "", fmt.Errorf("打开图片失败：%v", err)
		}
		defer imgFile.Close()

		originalImg, _, err := image.Decode(imgFile)
		if err != nil {
			return "", fmt.Errorf("解码图片失败：%v", err)
		}

		resizedImg := resizeImage(originalImg, 960, 300)

		// 保存调整后的图片到临时路径
		tmpResizedPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s_resized.jpg", selMap[i]))
		tmpResizedFile, err := os.Create(tmpResizedPath)
		if err != nil {
			return "", fmt.Errorf("创建临时缩放图片文件失败：%v", err)
		}
		defer tmpResizedFile.Close()

		err = jpeg.Encode(tmpResizedFile, resizedImg, nil)
		if err != nil {
			return "", fmt.Errorf("保存缩放图片失败：%v", err)
		}

		tmpImgPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s_%s.jpg", selMap[i], current.Code))
		endTimeStr := time.Time(current.EndTime).Format("2006-01-02 15:04:05")
		remaining := time.Until(time.Time(current.EndTime))
		remainingStr := formatDuration(remaining)

		textLines := []tools.TextLine{
			{Text: GetModeName(selMap[i]), Size: 50, X: 20, Y: 60, FontPath: filepath.Join(assetFontDir, "海报粗圆体.ttf")},
			{Text: GetMapName(current.Code), Size: 80, X: 20, Y: 170, FontPath: filepath.Join(assetFontDir, "海报粗圆体.ttf")},
			{Text: fmt.Sprintf("结束时间：%s（%s）", endTimeStr, remainingStr), Size: 30, X: 20, Y: 220, FontPath: filepath.Join(assetFontDir, "海报粗圆体.ttf")},
			{Text: fmt.Sprintf("下一轮换：%s", GetMapName(next.Code)), Size: 30, X: 20, Y: 260, FontPath: filepath.Join(assetFontDir, "海报粗圆体.ttf")},
		}

		err = tools.AddTextToImage(tmpResizedPath, tmpImgPath, textLines)
		if err != nil {
			return "", fmt.Errorf("添加文本到图片失败：%v", err)
		}

		imgFile, err = os.Open(tmpImgPath)
		if err != nil {
			return "", fmt.Errorf("打开图片失败：%v", err)
		}
		defer imgFile.Close()

		img, _, err := image.Decode(imgFile)
		if err != nil {
			return "", fmt.Errorf("解码图片失败：%v", err)
		}

		images = append(images, img)
	}

	finalImg := image.NewRGBA(image.Rect(0, 0, 960, 900))
	for i, img := range images {
		draw.Draw(finalImg, image.Rect(0, i*300, 960, (i+1)*300), img, image.Point{}, draw.Src)
	}

	finalImgPath := filepath.Join(os.TempDir(), "map_ok.jpg")
	finalImgFile, err := os.Create(finalImgPath)
	if err != nil {
		return "", fmt.Errorf("创建最终图片文件失败：%v", err)
	}
	defer finalImgFile.Close()

	err = jpeg.Encode(finalImgFile, finalImg, nil)
	if err != nil {
		return "", fmt.Errorf("保存最终图片失败：%v", err)
	}

	return finalImgPath, nil
}

// resizeImage 调整图片大小
func resizeImage(img image.Image, width, height int) image.Image {
	bounds := img.Bounds()
	ratio := float64(bounds.Dx()) / float64(bounds.Dy())
	newWidth := int(float64(height) * ratio)
	if newWidth > width {
		newWidth = width
		height = int(float64(newWidth) / ratio)
	}

	resizedImg := image.NewRGBA(image.Rect(0, 0, newWidth, height))
	// 使用 golang.org/x/image/draw 的 NearestNeighbor
	draw.NearestNeighbor.Scale(resizedImg, resizedImg.Bounds(), img, img.Bounds(), draw.Over, nil)
	return resizedImg
}
func CacheAllImage(mapRotate MapRotate) {
	//  遍历下载地图图片
	for _, mapRotateInfo := range getMapRotate(mapRotate) {
		for _, aMapInfo := range getMapInfo(mapRotateInfo) {
			mapAssetURL := aMapInfo.Asset
			if mapAssetURL == "" {
				botlog.Warn("获取地图轮换数据失败")
				continue
			}
			// 缓存图片到本地
			cachePath, err := CacheImage(mapAssetURL)
			if err != nil {
				botlog.Warnf("缓存地图图片失败,图片路径：%s, err:%v", cachePath, err)
				continue
			}
		}
	}

}
func GetMapResult() (string, error) {
	mapRotate, err := GetMapRotate()
	if err != nil {
		botlog.Warnf("获取地图轮换失败: %v", err)
		return "", fmt.Errorf("获取地图轮换失败: %v", err)
	}
	CacheAllImage(mapRotate)
	mapResultPath, err := GenerateMapImage()
	return mapResultPath, err
}
