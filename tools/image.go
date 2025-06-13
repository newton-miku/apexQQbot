package tools

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	botlog "github.com/tencent-connect/botgo/log"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// TextLine 表示一行文本及其属性
type TextLine struct {
	Text      string
	Size      float64
	Color     color.Color
	X, Y      int
	Alignment string // "left", "center", "right"
	FontPath  string //  字体文件路径
}

// 在图片上添加多行文本
func AddTextToImage(inputPath, outputPath string, textLines []TextLine) error {
	// 打开输入图片
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("无法打开输入文件: %v", err)
	}
	defer inputFile.Close()

	// 解码图片
	img, _, err := image.Decode(inputFile)
	if err != nil {
		return fmt.Errorf("无法解码图片: %v", err)
	}

	// 创建一个可编辑的图片副本
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, img, image.Point{}, draw.Src)

	// 为每行文本设置不同大小和颜色
	for _, line := range textLines {
		addSingleLine(dst, line)
	}

	// 创建输出文件
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("无法创建输出文件: %v", err)
	}
	defer outputFile.Close()

	// 根据输出文件扩展名选择编码格式
	ext := strings.ToLower(filepath.Ext(outputPath))
	switch ext {
	case ".jpg", ".jpeg":
		err = jpeg.Encode(outputFile, dst, &jpeg.Options{Quality: 95})
	case ".png":
		err = png.Encode(outputFile, dst)
	default:
		return fmt.Errorf("不支持的输出格式: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("无法编码图片: %v", err)
	}

	return nil
}
func loadFontFace(fontPath string, size float64) (font.Face, error) {
	if fontPath == "" {
		return basicfont.Face7x13, nil
	}

	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		botlog.Warnf("读取字体文件失败：%v", err)
		return basicfont.Face7x13, nil
	}

	ttFont, err := opentype.Parse(fontBytes)
	if err != nil {
		botlog.Warnf("解析字体失败：%v", err)
		return basicfont.Face7x13, nil
	}

	face, err := opentype.NewFace(ttFont, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		botlog.Warnf("创建字体Face失败：%v", err)
		return basicfont.Face7x13, nil
	}

	return face, nil
}

// 添加单行文本
func addSingleLine(img draw.Image, line TextLine) {
	// 设置文本颜色
	fg := line.Color
	if fg == nil {
		fg = color.White // 默认白色
	}

	// 创建一个字体上下文
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(fg),
		Face: basicfont.Face7x13,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(line.X * 64),
			Y: fixed.Int26_6(line.Y*64 + basicfont.Face7x13.Metrics().Ascent.Ceil()),
		},
	}
	// 如果指定了字体文件，则加载该字体
	d.Face, _ = loadFontFace(line.FontPath, line.Size)
	// 计算文本宽度以进行对齐
	width := 0
	for _, r := range line.Text {
		adv, ok := d.Face.GlyphAdvance(r)
		if !ok {
			continue
		}
		width += int(adv.Round())
	}

	if line.Alignment == "center" {
		d.Dot.X = fixed.Int26_6(line.X*64 - width/2)
	} else if line.Alignment == "right" {
		d.Dot.X = fixed.Int26_6(line.X*64 - width)
	}

	// 根据对齐方式调整X坐标
	if line.Alignment == "center" {
		d.Dot.X = fixed.Int26_6(line.X*64 - width/2)
	} else if line.Alignment == "right" {
		d.Dot.X = fixed.Int26_6(line.X*64 - width)
	}

	// 绘制文本
	d.DrawString(line.Text)
}

func testImageTextOverlay() {
	if len(os.Args) < 3 {
		fmt.Println("用法: go run image.go <输入图片路径> <输出图片路径>")
		return
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]

	// 定义多行文本及其属性
	textLines := []TextLine{
		{Text: "大标题", Size: 36, Color: color.RGBA{255, 255, 0, 255}, X: 300, Y: 100, Alignment: "center"},
		{Text: "中等大小的副标题", Size: 24, Color: color.RGBA{255, 0, 0, 255}, X: 300, Y: 150, Alignment: "center"},
		{Text: "这是正文内容", Size: 16, Color: color.RGBA{255, 255, 255, 255}, X: 300, Y: 200, Alignment: "center"},
		{Text: "这是底部注释", Size: 12, Color: color.RGBA{200, 200, 200, 255}, X: 300, Y: 250, Alignment: "center"},
	}

	err := AddTextToImage(inputPath, outputPath, textLines)
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}

	fmt.Printf("成功在图片上添加文本，输出文件: %s\n", outputPath)
}
