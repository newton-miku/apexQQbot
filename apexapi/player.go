package apexapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/newton-miku/apexQQbot/tools"
)

// GetData 发起API请求并处理响应
func GetPlayerData(EAID string) (string, error) {
	params := url.Values{}
	// params.Add("player", EAID)
	// params.Add("platform", "PC")

	params.Add("userName", EAID)
	params.Add("userPlatform", "PC")
	params.Add("qt", "stats-single-legend")
	urlStr := "https://lil2-gateway.apexlegendsstatus.com/gateway.php?" + params.Encode()
	// urlStr := "https://api.mozambiquehe.re/bridge?" + params.Encode()

	if urlStr == "" {
		return "", ErrEmptyURL
	}

	if ApiConf.ApiToken == "" {
		return "", ErrEmptyAPIToken
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer([]byte("")))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrRequestCreateFailed, err)
	}
	// req.Header.Set("Authorization", ApiConf.ApiToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	limitReader := io.LimitReader(resp.Body, 64<<10) // 最大 64K
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return "", ErrReadResponseFailed
	}

	log.Printf("请求 %s 时返回的状态码: 【%d】", urlStr, resp.StatusCode)

	if resp.StatusCode == http.StatusOK {
		var tempJson map[string]interface{}
		var statRes struct {
			StatsAPI map[string]interface{} `json:"statsAPI"`
		}
		err := json.Unmarshal(body, &statRes)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidJSON, err)
		}

		tempJson = statRes.StatsAPI

		if errMsg, ok := tempJson["Error"]; ok {
			if errorMsgStr, ok := errMsg.(string); ok {
				log.Printf("错误数据：%v", tempJson)
				if strings.Contains(errorMsgStr, "Player") && strings.Contains(errorMsgStr, "not found") {
					return "", ErrAPIReturnedError("未找到该玩家信息，请检查EAID的正确性")
				} else {
					return "", ErrAPIReturnedError(errorMsgStr)
				}
			} else {
				return "", fmt.Errorf("错误字段 'Error' 不是字符串类型：%T", errMsg)
			}
		}
		body, _ = json.Marshal(tempJson)
		return string(body), nil
	}

	if strings.Contains(string(body), "API key doesn't exist !") && resp.StatusCode == 404 {
		return "", ErrWrongAPIToken
	}
	return "", ErrStatusCode(resp.StatusCode)
}

type DisplayChangedOption struct {
	LastScore int
	LastTime  time.Time
}

// DisplayPlayerData 美观地显示玩家数据
func DisplayPlayerData(data string, change ...DisplayChangedOption) string {
	var playerData map[string]interface{}
	err := json.Unmarshal([]byte(data), &playerData)
	if err != nil {
		return fmt.Sprintf("解析玩家数据出错: %v\n", err)
	}

	// 提取基本信息
	globalData, ok := playerData["global"].(map[string]interface{})
	if !ok {
		return "未找到玩家基本信息"
	}

	// 提取传奇信息
	legendsData, ok := playerData["legends"].(map[string]interface{})
	if !ok {
		return "未找到传奇信息"
	}

	selectedLegend, ok := legendsData["selected"].(map[string]interface{})
	if !ok {
		return "未找到当前选择的传奇"
	}

	legendData, ok := selectedLegend["data"].([]interface{})
	if !ok {
		return "未找到传奇数据"
	}

	// 构建输出字符串
	var output strings.Builder
	output.WriteString("\n== Apex Legends 玩家信息 ==\n")
	output.WriteString(fmt.Sprintf("玩家名称: %s\n", globalData["name"]))
	output.WriteString(fmt.Sprintf("UID: %v\n", globalData["uid"]))
	output.WriteString(fmt.Sprintf("平台: %s\n", globalData["platform"]))
	output.WriteString(fmt.Sprintf("等级: %.0f\n", globalData["level"].(float64)))

	rankData, ok := globalData["rank"].(map[string]interface{})
	if ok {
		output.WriteString(fmt.Sprintf("段位: %s %v\n", rankData["rankName"], int(rankData["rankDiv"].(float64))))
		output.WriteString(fmt.Sprintf("段位分数: %.0f\n", rankData["rankScore"].(float64)))
		if len(change) > 0 {
			deltaScore := int(rankData["rankScore"].(float64)) - change[0].LastScore
			if deltaScore != 0 {
				output.WriteString(fmt.Sprintf("段位分数变化: %+d\n", deltaScore))
				output.WriteString(fmt.Sprintf("当前用户上次查询时间: %s\n", change[0].LastTime.Format("2006-01-02 15:04:05")))
			}
		}
	}
	legendsNamePath := "asset/legends.json"
	// 创建翻译器
	legendTrans, err := tools.NewTranslator(legendsNamePath)
	if err != nil {
		log.Printf("初始化翻译器失败：%v\n", err)
	}
	defer legendTrans.Close()
	legendName := legendTrans.Translate(selectedLegend["LegendName"].(string))
	output.WriteString(fmt.Sprintf("\n当前选择的传奇: %s\n", legendName))

	output.WriteString("传奇数据:\n")
	for i, stat := range legendData {
		statData, ok := stat.(map[string]interface{})
		if !ok {
			continue
		}
		output.WriteString(fmt.Sprintf("  %d. %s: %v\n", i+1, statData["name"], statData["value"]))
	}

	output.WriteString("=========================\n")

	return output.String()
}

func GetPlayerRank(data string) (int, error) {
	var playerData map[string]interface{}
	err := json.Unmarshal([]byte(data), &playerData)
	if err != nil {
		return 0, fmt.Errorf("解析玩家数据出错: %v", err)
	}

	// 提取基本信息
	globalData, ok := playerData["global"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("未找到玩家基本信息")
	}
	rankData, ok := globalData["rank"].(map[string]interface{})
	if ok {
		return int(rankData["rankScore"].(float64)), nil
	}
	return 0, fmt.Errorf("未找到段位信息")
}
