package apexapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/newton-miku/apexQQbot/tools"
)

// 线程安全的翻译器初始化
var (
	legendsTranslator *tools.Translator
	legendsDictPath   = "./asset/legends.json"
	translatorOnce    sync.Once
)

func getLegendsTranslator() *tools.Translator {
	translatorOnce.Do(func() {
		legendTrans, err := tools.NewTranslator(legendsDictPath)
		if err != nil {
			// 静默失败，使用默认行为
			return
		}
		legendsTranslator = legendTrans
	})
	return legendsTranslator
}

// ============ 数据结构定义 ============

// PlayerResponse API 响应结构
type PlayerResponse struct {
	Global  GlobalInfo            `json:"global"`
	Legends map[string]LegendInfo `json:"legends"`
}

// GlobalInfo 全局玩家信息
type GlobalInfo struct {
	Name     string    `json:"name"`
	UID      any       `json:"uid"`       // 支持 string 或 int64
	Platform string    `json:"platform"`
	Level    float64   `json:"level"`
	Rank     RankInfo  `json:"rank"`
}

// RankInfo 段位信息
type RankInfo struct {
	RankName  string  `json:"rankName"`
	RankDiv   int     `json:"rankDiv"`
	RankScore float64 `json:"rankScore"`
}

// LegendInfo 传奇信息
type LegendInfo struct {
	Selected LegendSelected `json:"selected"`
}

// LegendSelected 当前选择的传奇
type LegendSelected struct {
	LegendName string                 `json:"LegendName"`
	Data      []LegendStatItem       `json:"data"`
	ImgAssets LegendImgAssets        `json:"ImgAssets"`
}

// LegendImgAssets 传奇图片资源
type LegendImgAssets struct {
	Icon string `json:"icon"`
	Tab  string `json:"tab"`
}

// LegendStatItem 传奇数据项
type LegendStatItem struct {
	Name  string `json:"name"`
	Value any    `json:"value"` // 支持字符串或数字
}

// DisplayChangedOption 显示变化选项
type DisplayChangedOption struct {
	LastScore int
	LastTime  time.Time
}

// ============ API 调用函数 ============

// GetPlayerData 获取玩家数据（返回结构体）
func GetPlayerData(ctx context.Context, EAID string) (*PlayerResponse, error) {
	params := url.Values{}
	params.Add("userName", EAID)
	params.Add("userPlatform", "PC")
	params.Add("qt", "stats-single-legend")
	urlStr := "https://lil2-gateway.apexlegendsstatus.com/gateway.php?" + params.Encode()

	if ApiConf.ApiToken == "" {
		return nil, ErrEmptyAPIToken
	}

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestCreateFailed, err)
	}

	resp, err := GetHTTPClient(10 * time.Second).Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	limitReader := io.LimitReader(resp.Body, 64<<10) // 最大 64K
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, ErrReadResponseFailed
	}

	if resp.StatusCode == http.StatusOK {
		var statRes struct {
			StatsAPI *PlayerResponse `json:"statsAPI"`
		}
		err := json.Unmarshal(body, &statRes)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
		}

		if statRes.StatsAPI == nil {
			return nil, fmt.Errorf("%w: statsAPI 为空", ErrInvalidJSON)
		}

		// 检查 API 返回的错误
		// 注意：这里需要根据实际的 API 响应结构调整
		return statRes.StatsAPI, nil
	}

	if strings.Contains(string(body), "API key doesn't exist !") && resp.StatusCode == 404 {
		return nil, ErrWrongAPIToken
	}
	return nil, ErrStatusCode(resp.StatusCode)
}

// GetPlayerRank 从结构体获取段位分数
func GetPlayerRank(player *PlayerResponse) (int, error) {
	if player == nil {
		return 0, fmt.Errorf("玩家数据为空")
	}
	return int(player.Global.Rank.RankScore), nil
}

// ============ 格式化输出函数 ============

// FormatPlayerData 美观地格式化玩家数据
func FormatPlayerData(player *PlayerResponse, change ...DisplayChangedOption) string {
	if player == nil {
		return "玩家数据为空"
	}

	var output strings.Builder
	output.WriteString("\n== Apex Legends 玩家信息 ==\n")

	// 玩家名称
	if player.Global.Name != "" {
		output.WriteString(fmt.Sprintf("玩家名称: %s\n", player.Global.Name))
	}

	// UID
	if player.Global.UID != nil {
		output.WriteString(fmt.Sprintf("UID: %v\n", player.Global.UID))
	}

	// 平台
	if player.Global.Platform != "" {
		output.WriteString(fmt.Sprintf("平台: %s\n", player.Global.Platform))
	}

	// 等级
	if player.Global.Level > 0 {
		output.WriteString(fmt.Sprintf("等级: %.0f\n", player.Global.Level))
	}

	// 段位信息
	if player.Global.Rank.RankName != "" {
		output.WriteString(fmt.Sprintf("段位: %s %v\n", player.Global.Rank.RankName, player.Global.Rank.RankDiv))
		output.WriteString(fmt.Sprintf("段位分数: %.0f\n", player.Global.Rank.RankScore))

		if len(change) > 0 {
			deltaScore := int(player.Global.Rank.RankScore) - change[0].LastScore
			if deltaScore != 0 {
				output.WriteString(fmt.Sprintf("段位分数变化: %+d\n", deltaScore))
				output.WriteString(fmt.Sprintf("当前用户上次查询时间: %s\n", change[0].LastTime.Format("2006-01-02 15:04:05")))
			}
		}
	}

	// 当前传奇
	if selected, ok := player.Legends["selected"]; ok {
		legendName := GetLegendName(selected.Selected.LegendName)
		output.WriteString(fmt.Sprintf("\n当前选择的传奇: %s\n", legendName))
	}

	// 传奇数据
	output.WriteString("传奇数据:\n")
	if selected, ok := player.Legends["selected"]; ok {
		for i, stat := range selected.Selected.Data {
			output.WriteString(fmt.Sprintf("  %d. %s: %v\n", i+1, stat.Name, stat.Value))
		}
	}

	output.WriteString("=========================\n")

	return output.String()
}

// GetLegendName 获取传奇名称（中文）
func GetLegendName(legendName string) string {
	trans := getLegendsTranslator()
	if trans == nil {
		return legendName
	}
	return trans.Translate(legendName)
}

// ============ 向后兼容的包装函数 ============

// DisplayPlayerData 向后兼容的显示函数（接受 JSON 字符串）
func DisplayPlayerData(data string, change ...DisplayChangedOption) string {
	var player PlayerResponse
	if err := json.Unmarshal([]byte(data), &player); err != nil {
		return fmt.Sprintf("解析玩家数据出错: %v\n", err)
	}
	return FormatPlayerData(&player, change...)
}

// GetPlayerRank 向后兼容的获取段位函数（接受 JSON 字符串）
func GetPlayerRankFromString(data string) (int, error) {
	var player PlayerResponse
	if err := json.Unmarshal([]byte(data), &player); err != nil {
		return 0, fmt.Errorf("解析玩家数据出错: %v", err)
	}
	return GetPlayerRank(&player)
}
