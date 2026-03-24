package apexapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ItemStoreClient 用于抓取 apexitemstore.com 数据
type ItemStoreClient struct {
	client    *http.Client
	baseURL   string
	nonce     string
	nonceTime time.Time
}

// NewItemStoreClient 创建客户端
func NewItemStoreClient() *ItemStoreClient {
	return &ItemStoreClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://apexitemstore.com",
	}
}

// fetchNonce 从主页获取 nonce
func (c *ItemStoreClient) fetchNonce() error {
	// 请求主页
	resp, err := c.client.Get(c.baseURL)
	if err != nil {
		return fmt.Errorf("fetch homepage failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body failed: %w", err)
	}

	// 从 HTML 中提取 nonce
	// 格式: "nonce":"19cc681867"
	re := regexp.MustCompile(`"nonce"\s*:\s*"([^"]+)"`)
	matches := re.FindSubmatch(body)
	if len(matches) < 2 {
		return fmt.Errorf("nonce not found in response")
	}

	c.nonce = string(matches[1])
	c.nonceTime = time.Now()
	return nil
}

// getNonce 获取有效的 nonce（带缓存）
func (c *ItemStoreClient) getNonce() (string, error) {
	// nonce 有效期约 12-24 小时，这里保守使用 10 小时
	if c.nonce != "" && time.Since(c.nonceTime) < 10*time.Hour {
		return c.nonce, nil
	}

	if err := c.fetchNonce(); err != nil {
		return "", err
	}
	return c.nonce, nil
}

// QueryNextEvent 调用 API 获取下一个活动
func (c *ItemStoreClient) QueryNextEvent(deadline string) (*EventResponse, error) {
	nonce, err := c.getNonce()
	if err != nil {
		return nil, err
	}

	// 构建 URL
	var sb strings.Builder
	sb.WriteString(c.baseURL)
	sb.WriteString("/wp-admin/admin-ajax.php?")

	params := fmt.Sprintf("action=scd_query_next_event&smartcountdown_nonce=%s&unique_ts=%d&deadline=&import_config=scd_easy_recurrence%%3A%%3A2&countdown_to_end=0&countup_limit=0", nonce, time.Now().UnixMilli())
	sb.WriteString(params)

	req, err := http.NewRequest("GET", sb.String(), nil)
	if err != nil {
		return nil, err
	}

	// 设置 Headers 模拟浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json, text/javascript, */*")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", c.baseURL+"/")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 检查是否需要刷新 nonce
	if strings.Contains(string(body), "invalid_nonce") {
		// nonce 失效，重新获取
		c.nonce = "" // 清除缓存
		return c.QueryNextEvent(deadline)
	}

	var result EventResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json failed: %w, body: %s", err, string(body))
	}

	return &result, nil
}

// EventResponse API 响应结构
type EventResponse struct {
	ErrCode int `json:"err_code"`
	Options struct {
		Deadline            string `json:"deadline"`
		Now                 int64  `json:"now"`
		CountupLimit        int    `json:"countup_limit"`
		CountdownQueryLimit int    `json:"countdown_query_limit"`
		ImportedTitle       string `json:"imported_title,omitempty"`
		IsCountdownToEnd    int    `json:"is_countdown_to_end,omitempty"`
	} `json:"options"`
}

// IsSuccess 检查响应是否成功
func (r *EventResponse) IsSuccess() bool {
	return r.ErrCode == 0
}
