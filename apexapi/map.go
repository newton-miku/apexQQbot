package apexapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/newton-miku/apexQQbot/tools"
	botlog "github.com/tencent-connect/botgo/log"
	"golang.org/x/image/draw"
)

// ============ 公共路径工具 ============

var (
	exeDir      string
	repoRoot    string
	assetDir    string
	assetPath   sync.Once
	pathInitErr error
)

// InitPaths 初始化路径（在程序启动时调用）
func InitPaths() {
	pathInitErr = initPaths()
}

// getPaths 确保路径已初始化
func getPaths() error {
	if pathInitErr != nil {
		return pathInitErr
	}
	assetPath.Do(func() {
		pathInitErr = initPaths()
	})
	return pathInitErr
}

// GetAssetPath 获取资源目录
func GetAssetPath() (string, error) {
	if err := getPaths(); err != nil {
		return "", err
	}
	return assetDir, nil
}

// GetCachePath 获取缓存目录
func GetCachePath() (string, error) {
	if err := getPaths(); err != nil {
		return "", err
	}
	return filepath.Join(assetDir, "Map"), nil
}

// ============ HTTP Client 复用 ============
// 已移动到 http.go，使用 GetHTTPClient() 函数

// ============ JSON 时间类型 ============

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

// ============ 地图数据结构 ============

// 单张地图的信息
type MapInfo struct {
	Name      string   `json:"map"`
	Code      string   `json:"code"`
	StartTime UnixTime `json:"start"`
	EndTime   UnixTime `json:"end"`
	Asset     string
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

func getMapRotateData(mr MapRotate) []MapRotateInfo {
	return []MapRotateInfo{
		mr.Battle_royale,
		mr.Ranked,
		mr.Ltm,
	}
}

// ============ 缓存与线程安全 ============

var (
	cachedMapRotate MapRotate
	cacheExpiresAt  time.Time
	mapCacheLock    sync.RWMutex
	mapCacheOnce    sync.Once
	mapCacheInitErr error
)

// GetEarliestEndTime 获取所有 Current Map 中最早的 EndTime
func GetEarliestEndTime(mr MapRotate) time.Time {
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

// RefreshMapCache 刷新地图缓存
func RefreshMapCache() {
	mapCacheOnce.Do(func() {
		_, mapCacheInitErr = GetMapRotateFromAPI()
	})
}

// GetMapRotate 获取地图轮换（带缓存）
func GetMapRotate() (MapRotate, error) {
	// 快速路径：检查缓存是否有效
	mapCacheLock.RLock()
	if !cacheExpiresAt.IsZero() && time.Now().Before(cacheExpiresAt) {
		result := cachedMapRotate
		mapCacheLock.RUnlock()
		return result, nil
	}
	mapCacheLock.RUnlock()

	// 缓存无效，重新获取
	return GetMapRotateFromAPI()
}

// GetMapRotateFromAPI 从 API 获取地图轮换（不带缓存）
func GetMapRotateFromAPI() (MapRotate, error) {
	client := GetHTTPClient(15 * time.Second)
	req, err := http.NewRequest("GET", "https://lil2-gateway.apexlegendsstatus.com/gateway.php?qt=map", nil)
	if err != nil {
		return MapRotate{}, fmt.Errorf("%w: %v", ErrRequestCreateFailed, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return MapRotate{}, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	limitReader := io.LimitReader(resp.Body, 5<<10)
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return MapRotate{}, ErrReadResponseFailed
	}

	if resp.StatusCode == http.StatusOK {
		var mapRotate MapRotate
		var mapRaw struct {
			MapRotate MapRotate `json:"rotation"`
		}

		if err := json.Unmarshal(body, &mapRaw); err != nil {
			return MapRotate{}, ErrInvalidJSON
		}
		mapRotate = mapRaw.MapRotate

		// 更新缓存（写锁）
		mapCacheLock.Lock()
		cachedMapRotate = mapRotate
		cacheExpiresAt = GetEarliestEndTime(mapRotate)
		mapCacheLock.Unlock()

		return mapRotate, nil
	}

	return MapRotate{}, ErrStatusCode(resp.StatusCode)
}

// ForceRefreshMapCache 强制刷新地图缓存
func ForceRefreshMapCache() (MapRotate, error) {
	mapCacheLock.Lock()
	cachedMapRotate = MapRotate{}
	cacheExpiresAt = time.Time{}
	mapCacheLock.Unlock()

	mapCacheOnce = sync.Once{} // 重置 Once
	RefreshMapCache()
	return cachedMapRotate, mapCacheInitErr
}

// ============ 翻译器管理 ============

var (
	mapTranslator  *tools.Translator
	modeTranslator *tools.Translator
	translatorLock sync.RWMutex
	mapDictPath    string
	modeDictPath   string
)

// GetMapName 获取地图中文名（线程安全）
func GetMapName(code string) string {
	translatorLock.RLock()
	trans := mapTranslator
	translatorLock.RUnlock()

	if trans == nil {
		t, err := tools.NewTranslator(mapDictPath)
		if err == nil {
			translatorLock.Lock()
			mapTranslator = t
			translatorLock.Unlock()
			return t.Translate(code)
		}
		return code
	}
	return trans.Translate(code)
}

// GetModeName 获取模式中文名（线程安全）
func GetModeName(code string) string {
	translatorLock.RLock()
	trans := modeTranslator
	translatorLock.RUnlock()

	if trans == nil {
		t, err := tools.NewTranslator(modeDictPath)
		if err == nil {
			translatorLock.Lock()
			modeTranslator = t
			translatorLock.Unlock()
			return t.Translate(code)
		}
		return code
	}
	return trans.Translate(code)
}

// ============ 图片缓存 ============

// CacheImage 下载并缓存图片（线程安全）
func CacheImage(urlLink string) (string, error) {
	u, err := url.Parse(urlLink)
	if err != nil {
		return "", fmt.Errorf("%w: 解析URL失败: %v", ErrRequestCreateFailed, err)
	}

	filename := filepath.Base(u.Path)
	cacheDir, err := GetCachePath()
	if err != nil {
		return "", fmt.Errorf("获取缓存目录失败: %w", err)
	}

	cachePath := filepath.Join(cacheDir, filename)

	// 确保缓存目录存在
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("创建缓存目录失败: %w", err)
	}

	// 检查是否已有缓存
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	// 下载图片
	client := GetHTTPClient(10 * time.Second)
	req, err := http.NewRequest("GET", urlLink, nil)
	if err != nil {
		return "", fmt.Errorf("%w: 创建请求失败: %v", ErrRequestCreateFailed, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: 下载失败: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", ErrStatusCode(resp.StatusCode)
	}

	// 创建并写入文件（带 O_EXCL 防止竞态）
	outFile, err := os.OpenFile(cachePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		// 文件可能已由其他协程创建
		if errors.Is(err, os.ErrExist) {
			return cachePath, nil
		}
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer func() {
		_ = outFile.Close()
		if err != nil {
			_ = os.Remove(cachePath)
		}
	}()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return cachePath, nil
}

// CacheAllImage 缓存所有地图图片
func CacheAllImage(mapRotate MapRotate) {
	for _, mapRotateInfo := range getMapRotateData(mapRotate) {
		for _, aMapInfo := range getMapInfo(mapRotateInfo) {
			if aMapInfo.Asset == "" {
				botlog.Warn("获取地图轮换数据失败：空Asset URL")
				continue
			}

			cachePath, err := CacheImage(aMapInfo.Asset)
			if err != nil {
				botlog.Warnf("缓存地图图片失败: %v", err)
				continue
			}
			_ = cachePath
		}
	}
}

// ============ 图片生成 ============

// FormatDuration 格式化持续时间为 "Xd HH:mm:ss"
func FormatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if days > 0 {
		return fmt.Sprintf("%dd %02d:%02d:%02d", days, h, m, s)
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// ResizeImage 调整图片大小
func ResizeImage(img image.Image, width, height int, options ...bool) image.Image {
	keepRatio := false
	if len(options) > 0 {
		keepRatio = options[0]
	}
	if keepRatio {
		bounds := img.Bounds()
		ratio := float64(bounds.Dx()) / float64(bounds.Dy())
		newWidth := int(float64(height) * ratio)
		if newWidth > width {
			newWidth = width
			height = int(float64(newWidth) / ratio)
		}
		width = newWidth
	}
	resizedImg := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(resizedImg, resizedImg.Bounds(), img, img.Bounds(), draw.Over, nil)
	return resizedImg
}

// GenerateMapImage 生成地图轮换信息图片
func GenerateMapImage() (string, error) {
	mapRotate, err := GetMapRotate()
	if err != nil {
		return "", fmt.Errorf("获取地图轮换信息失败: %w", err)
	}

	// 获取商店倒计时
	storeCountdown, _ := GetStoreCountdown()

	assetDir, err := GetAssetPath()
	if err != nil {
		return "", fmt.Errorf("获取资源目录失败: %w", err)
	}
	fontPath := filepath.Join(assetDir, "Font", "海报粗圆体.ttf")

	selMap := []string{"battle_royale", "ranked", "ltm"}
	images := make([]image.Image, 0, 3)

	for i, modeMap := range getMapRotateData(mapRotate) {
		current := modeMap.Current
		next := modeMap.Next

		// 缓存并读取当前地图图片
		imgPath, err := CacheImage(current.Asset)
		if err != nil {
			return "", fmt.Errorf("缓存图片失败: %w", err)
		}

		imgFile, err := os.Open(imgPath)
		if err != nil {
			return "", fmt.Errorf("打开图片失败: %w", err)
		}
		origImg, _, err := image.Decode(imgFile)
		_ = imgFile.Close()
		if err != nil {
			return "", fmt.Errorf("解码图片失败: %w", err)
		}

		// 缩放到目标尺寸
		resizedImg := ResizeImage(origImg, 960, 300)

		// 创建可编辑副本
		dst := image.NewRGBA(resizedImg.Bounds())
		draw.Draw(dst, dst.Bounds(), resizedImg, image.Point{}, draw.Src)

		// 构造文本信息
		endTimeStr := time.Time(current.EndTime).Format("2006-01-02 15:04:05")
		remaining := time.Until(time.Time(current.EndTime))
		if remaining < 0 {
			remaining = 0
		}
		remainingStr := FormatDuration(remaining)

		textLines := []tools.TextLine{
			{Text: GetModeName(selMap[i]), Size: 50, X: 20, Y: 60, FontPath: fontPath},
			{Text: GetMapName(current.Code), Size: 80, X: 20, Y: 150, FontPath: fontPath},
			{Text: fmt.Sprintf("结束时间：%s（%s）", endTimeStr, remainingStr), Size: 30, X: 20, Y: 210, FontPath: fontPath},
			{Text: fmt.Sprintf("下一轮换：%s", GetMapName(next.Code)), Size: 30, X: 20, Y: 250, FontPath: fontPath},
		}

		// 直接在内存图片上绘制文本
		tools.AddTextToImageInPlace(dst, textLines)

		images = append(images, dst)
	}

	// 创建最终图片（添加倒计时栏在顶部）
	const headerHeight = 80
	finalImg := image.NewRGBA(image.Rect(0, 0, 960, 900+headerHeight))

	// 绘制倒计时栏背景
	for y := 0; y < headerHeight; y++ {
		for x := 0; x < 960; x++ {
			finalImg.Set(x, y, color.RGBA{30, 30, 40, 255})
		}
	}

	// 添加倒计时文本 - 布局：商店更新倒计时：[时间]                        商店更新：MM-DD HH:mm
	headerLines := []tools.TextLine{}

	// 如果有有效的倒计时信息
	if storeCountdown != nil && storeCountdown.IsValid() {
		// 左侧标签
		headerLines = append(headerLines, tools.TextLine{
			Text:     "商店更新倒计时：",
			Size:     32,
			X:        30,
			Y:        50,
			FontPath: fontPath,
			Color:    color.RGBA{200, 200, 200, 255},
		})
		// 倒计时时间（紧跟标签后面）
		headerLines = append(headerLines, tools.TextLine{
			Text:     storeCountdown.String(),
			Size:     42,
			X:        320,
			Y:        55,
			FontPath: fontPath,
			Color:    color.RGBA{255, 200, 100, 255},
		})
		// 商店更新时间放在最右侧
		endStr := storeCountdown.Deadline.Format("01-02 15:04")
		headerLines = append(headerLines, tools.TextLine{
			Text:      fmt.Sprintf("商店更新：%s", endStr),
			Size:      28,
			X:         930,
			Y:         50,
			FontPath:  fontPath,
			Color:     color.RGBA{200, 200, 200, 255},
			Alignment: "right",
		})
	} else {
		// 无数据时显示
		headerLines = append(headerLines, tools.TextLine{
			Text:     "商店更新还有：",
			Size:     32,
			X:        30,
			Y:        50,
			FontPath: fontPath,
			Color:    color.RGBA{200, 200, 200, 255},
		})
		headerLines = append(headerLines, tools.TextLine{
			Text:     "暂无信息",
			Size:     32,
			X:        260,
			Y:        50,
			FontPath: fontPath,
			Color:    color.RGBA{150, 150, 150, 255},
		})
	}

	tools.AddTextToImageInPlace(finalImg, headerLines)

	// 竖向拼接三张模式图片（在倒计时栏下方）
	for i, img := range images {
		dstRect := image.Rect(0, headerHeight+i*300, 960, headerHeight+(i+1)*300)
		draw.CatmullRom.Scale(finalImg, dstRect, img, img.Bounds(), draw.Over, nil)
	}

	// 保存到临时目录
	cacheDir, err := GetCachePath()
	if err != nil {
		return "", fmt.Errorf("获取缓存目录失败: %w", err)
	}
	finalImgPath := filepath.Join(cacheDir, "map_result.jpg")

	finalImgFile, err := os.Create(finalImgPath)
	if err != nil {
		return "", fmt.Errorf("创建最终图片文件失败: %w", err)
	}
	defer finalImgFile.Close()

	if err := jpeg.Encode(finalImgFile, finalImg, &jpeg.Options{Quality: 95}); err != nil {
		return "", fmt.Errorf("保存最终图片失败: %w", err)
	}

	return finalImgPath, nil
}

// GetMapResult 获取地图结果（图片路径）
func GetMapResult() (string, error) {
	mapRotate, err := GetMapRotate()
	if err != nil {
		botlog.Warnf("获取地图轮换失败: %v", err)
		return "", err
	}

	CacheAllImage(mapRotate)

	mapResultPath, err := GenerateMapImage()
	if err != nil {
		botlog.Warnf("生成地图图片失败: %v", err)
		return "", err
	}

	return mapResultPath, nil
}

// ============ 路径初始化 ============

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func initPaths() error {
	// 优先使用可执行文件所在目录
	exe, err := os.Executable()
	if err != nil {
		exeDir = ""
	} else {
		exeDir = filepath.Dir(exe)
	}

	assetCandidate := ""
	if exeDir != "" {
		assetCandidate = filepath.Join(exeDir, "asset")
	}

	// 如果可执行文件目录下不存在 asset，则使用源码目录
	if assetCandidate == "" || !pathExists(assetCandidate) {
		_, thisFile, _, _ := runtime.Caller(0)
		apexapiDir := filepath.Dir(thisFile)
		repoRoot = filepath.Dir(apexapiDir)
		assetCandidate = filepath.Join(repoRoot, "asset")
	}

	// 如果仍不存在，尝试当前工作目录
	if assetCandidate == "" || !pathExists(assetCandidate) {
		wd, _ := os.Getwd()
		repoRoot = filepath.Dir(wd)
		assetCandidate = filepath.Join(wd, "asset")
	}

	// 最后检查
	if assetCandidate == "" || !pathExists(assetCandidate) {
		return fmt.Errorf("无法找到资源目录: %s", assetCandidate)
	}

	assetDir = assetCandidate
	mapDictPath = filepath.Join(assetDir, "map_dict.json")
	modeDictPath = filepath.Join(assetDir, "mode_dict.json")

	return nil
}

func init() {
	InitPaths()
}
