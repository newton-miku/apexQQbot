package apexapi

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/newton-miku/apexQQbot/tools"
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
		// 因api网关位于海外，请求时间可能较长
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", "https://lil2-gateway.apexlegendsstatus.com/gateway.php?qt=map", nil)
	// req, err := http.NewRequest("GET", "https://api.mozambiquehe.re/maprotation?version=2", nil)
	if err != nil {
		return MapRotate{}, fmt.Errorf("%w: %v", ErrRequestCreateFailed, err)
	}
	// req.Header.Set("Authorization", ApiConf.ApiToken)

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
		var mapRaw struct {
			MapRotate MapRotate `json:"rotation"`
		}

		err := json.Unmarshal(body, &mapRaw)
		if err != nil {
			botlog.Debugf("JSON解析错误: %v", err)
			return MapRotate{}, ErrInvalidJSON
		}
		mapRotate = mapRaw.MapRotate

		// 更新缓存和过期时间
		cachedMapRotate = mapRotate
		cacheExpiresAt = getEarliestEndTime(mapRotate)

		return mapRotate, nil
	}
	return MapRotate{}, ErrStatusCode(resp.StatusCode)
}

var (
	// 使用通用翻译器
	mapTranslator  *tools.Translator
	modeTranslator *tools.Translator
	mapDictPath    = "./asset/map_dict.json"
	modeDictPath   = "./asset/mode_dict.json"
	assetDir       = "./asset/"
	assetFontDir   = "./asset/Font"
	assetCacheDir  = "./asset/Map"
)

// 获取地图中文名（线程安全）
func GetMapName(code string) string {
	// 如果翻译器不存在，创建一个
	if mapTranslator == nil {
		// 创建翻译器
		mapTrans, err := tools.NewTranslator(mapDictPath)
		if err != nil {
			botlog.Debugf("初始化翻译器失败：%v\n", err)
			return code // 缺失返回code
		}
		mapTranslator = mapTrans
	}
	return mapTranslator.Translate(code)
}

// 获取模式中文名（线程安全）
func GetModeName(code string) string {
	// 如果翻译器不存在，创建一个
	if modeTranslator == nil {
		// 创建翻译器
		modeTrans, err := tools.NewTranslator(modeDictPath)
		if err != nil {
			botlog.Debugf("初始化翻译器失败：%v\n", err)
			return code // 缺失返回code
		}
		modeTranslator = modeTrans
	}
	return modeTranslator.Translate(code)
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

// GenerateMapImage 生成地图轮换信息图片（优化：纯内存管线，避免临时文件I/O）
func GenerateMapImage() (string, error) {
	mapRotate, err := GetMapRotate()
	if err != nil {
		return "", fmt.Errorf("获取地图轮换信息失败：%v", err)
	}

	// 确保翻译器已初始化
	if mapTranslator == nil {
		if t, err := tools.NewTranslator(mapDictPath); err == nil {
			mapTranslator = t
		}
	}
	if modeTranslator == nil {
		if t, err := tools.NewTranslator(modeDictPath); err == nil {
			modeTranslator = t
		}
	}

	selMap := []string{"battle_royale", "ranked", "ltm"}
	images := make([]image.Image, 0, 3)

	for i, modeMap := range getMapRotate(mapRotate) {
		current := modeMap.Current
		next := modeMap.Next

		// 缓存并读取当前地图图片
		imgPath, err := CacheImage(current.Asset)
		if err != nil {
			return "", fmt.Errorf("缓存图片失败：%v", err)
		}

		imgFile, err := os.Open(imgPath)
		if err != nil {
			return "", fmt.Errorf("打开图片失败：%v", err)
		}
		origImg, _, err := image.Decode(imgFile)
		_ = imgFile.Close()
		if err != nil {
			return "", fmt.Errorf("解码图片失败：%v", err)
		}

		// 缩放到目标尺寸
		resizedImg := resizeImage(origImg, 960, 300)

		// 创建可编辑副本（内存中）
		dst := image.NewRGBA(resizedImg.Bounds())
		// 直接拷贝缩放后的图像到目标
		draw.Draw(dst, dst.Bounds(), resizedImg, image.Point{}, draw.Src)

		// 构造文本信息
		endTimeStr := time.Time(current.EndTime).Format("2006-01-02 15:04:05")
		remaining := time.Until(time.Time(current.EndTime))
		if remaining < 0 {
			remaining = 0
		}
		remainingStr := formatDuration(remaining)

		textLines := []tools.TextLine{
			{Text: GetModeName(selMap[i]), Size: 50, X: 20, Y: 60, FontPath: filepath.Join(assetFontDir, "海报粗圆体.ttf")},
			{Text: GetMapName(current.Code), Size: 80, X: 20, Y: 150, FontPath: filepath.Join(assetFontDir, "海报粗圆体.ttf")},
			{Text: fmt.Sprintf("结束时间：%s（%s）", endTimeStr, remainingStr), Size: 30, X: 20, Y: 210, FontPath: filepath.Join(assetFontDir, "海报粗圆体.ttf")},
			{Text: fmt.Sprintf("下一轮换：%s", GetMapName(next.Code)), Size: 30, X: 20, Y: 250, FontPath: filepath.Join(assetFontDir, "海报粗圆体.ttf")},
		}

		// 直接在内存图片上绘制文本
		tools.AddTextToImageInPlace(dst, textLines)

		images = append(images, dst)
	}

	// 竖向拼接三张模式图片
	finalImg := image.NewRGBA(image.Rect(0, 0, 960, 900))
	for i, img := range images {
		// 使用 CatmullRom.Scale 进行拷贝（无需额外导入标准库 image/draw）
		dstRect := image.Rect(0, i*300, 960, (i+1)*300)
		draw.CatmullRom.Scale(finalImg, dstRect, img, img.Bounds(), draw.Over, nil)
	}

	finalImgPath := filepath.Join(os.TempDir(), "map_ok.jpg")
	finalImgFile, err := os.Create(finalImgPath)
	if err != nil {
		return "", fmt.Errorf("创建最终图片文件失败：%v", err)
	}
	defer finalImgFile.Close()

	// 使用较高质量输出
	err = jpeg.Encode(finalImgFile, finalImg, &jpeg.Options{Quality: 95})
	if err != nil {
		return "", fmt.Errorf("保存最终图片失败：%v", err)
	}

	return finalImgPath, nil
}

// resizeImage 调整图片大小
func resizeImage(img image.Image, width, height int, options ...bool) image.Image {
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
