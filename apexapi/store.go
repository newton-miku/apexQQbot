package apexapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

const (
	storeAPIURL = "https://apexitemstore.com/wp-admin/admin-ajax.php"
)

// StoreEventResponse 商店事件响应
type StoreEventResponse struct {
	ErrCode int    `json:"err_code"`
	ErrMsg  string `json:"err_msg"`
	Options struct {
		ImportConfig        string `json:"import_config"`
		CountupLimit        int    `json:"countup_limit"`
		CountdownToEnd      int    `json:"countdown_to_end"`
		ImportedTitle       string `json:"imported_title"`
		Deadline            string `json:"deadline"`
		IsCountdownToEnd    int    `json:"is_countdown_to_end"`
		RedirectURL         string `json:"redirect_url"`
		CountdownQueryLimit int    `json:"countdown_query_limit"`
		Now                 int64  `json:"now"`
	} `json:"options"`
}

// StoreCountdown 商店倒计时信息
type StoreCountdown struct {
	Deadline   time.Time // 结束时间
	Now        time.Time // 服务器当前时间
	Title      string    // 事件标题
	IsCounting bool      // 是否正在倒计时
}

// RemainingTime 获取剩余时间
func (s *StoreCountdown) RemainingTime() time.Duration {
	if s.Deadline.IsZero() {
		return 0
	}
	remaining := time.Until(s.Deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsValid 检查倒计时是否有效
func (s *StoreCountdown) IsValid() bool {
	return !s.Deadline.IsZero() && s.RemainingTime() > 0
}

// String 格式化显示剩余时间
func (s *StoreCountdown) String() string {
	if !s.IsValid() {
		return "暂无倒计时信息"
	}
	return FormatDuration(s.RemainingTime())
}

var (
	cachedStoreCountdown *StoreCountdown
	storeCacheExpiresAt  time.Time
	storeCacheLock       sync.RWMutex
	storeCacheDuration   = 24 * time.Hour // 缓存1天

	// 动态 nonce 相关
	storeNonce     string
	storeNonceTime time.Time
	storeNonceLock sync.Mutex
)

// GetStoreCountdown 获取商店倒计时（带缓存）
func GetStoreCountdown() (*StoreCountdown, error) {
	// 快速路径：检查缓存是否有效
	storeCacheLock.RLock()
	if cachedStoreCountdown != nil && time.Now().Before(storeCacheExpiresAt) {
		result := cachedStoreCountdown
		storeCacheLock.RUnlock()
		return result, nil
	}
	storeCacheLock.RUnlock()

	// 缓存无效，重新获取
	return GetStoreCountdownFromAPI()
}

// getStoreNonce 获取有效的 nonce（带缓存）
func getStoreNonce() (string, error) {
	storeNonceLock.Lock()
	defer storeNonceLock.Unlock()

	// nonce 有效期约 12-24 小时，这里保守使用 10 小时
	if storeNonce != "" && time.Since(storeNonceTime) < 10*time.Hour {
		return storeNonce, nil
	}

	// 从主页获取新的 nonce
	client := GetHTTPClient(15 * time.Second)
	resp, err := client.Get("https://apexitemstore.com")
	if err != nil {
		return "", fmt.Errorf("fetch homepage failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body failed: %w", err)
	}

	// 从 HTML 中提取 nonce
	// 格式: "nonce":"19cc681867"
	re := regexp.MustCompile(`"nonce"\s*:\s*"([^"]+)"`)
	matches := re.FindSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("nonce not found in response")
	}

	storeNonce = string(matches[1])
	storeNonceTime = time.Now()
	return storeNonce, nil
}
// GetStoreCountdownFromAPI 从 API 获取商店倒计时
func GetStoreCountdownFromAPI() (*StoreCountdown, error) {
	// 获取动态 nonce
	nonce, err := getStoreNonce()
	if err != nil {
		return nil, fmt.Errorf("获取 nonce 失败: %w", err)
	}

	client := GetHTTPClient(15 * time.Second)

	// 构建请求 URL
	reqURL := fmt.Sprintf("%s?action=scd_query_next_event&smartcountdown_nonce=%s&unique_ts=%d&deadline=&import_config=scd_easy_recurrence%%3A%%3A2&countdown_to_end=0&countup_limit=0",
		storeAPIURL, nonce, time.Now().UnixMilli())

	req, err := http.NewRequestWithContext(context.Background(), "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestCreateFailed, err)
	}

	// 设置请求头
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Priority", "u=1, i")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", "https://apexitemstore.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrStatusCode(resp.StatusCode)
	}

	limitReader := io.LimitReader(resp.Body, 10<<10)
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, ErrReadResponseFailed
	}

	// 检查 nonce 是否失效
	if string(body) == "-1" || len(body) < 2 {
		// nonce 失效，清除缓存并重试
		storeNonceLock.Lock()
		storeNonce = ""
		storeNonceLock.Unlock()
		return GetStoreCountdownFromAPI()
	}

	var apiResp StoreEventResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, ErrInvalidJSON
	}

	if apiResp.ErrCode != 0 {
		return nil, fmt.Errorf("API 错误: %s", apiResp.ErrMsg)
	}

	// 解析截止时间（API 返回的是 UTC 时间）
	deadline, err := time.Parse(time.RFC3339, apiResp.Options.Deadline)
	if err != nil {
		return nil, fmt.Errorf("解析截止时间失败: %w", err)
	}
	// 转换为本地时间
	deadline = deadline.Local()

	countdown := &StoreCountdown{
		Deadline:   deadline,
		Now:        time.UnixMilli(apiResp.Options.Now).Local(),
		Title:      apiResp.Options.ImportedTitle,
		IsCounting: apiResp.Options.IsCountdownToEnd == 0 && !deadline.IsZero(),
	}

	// 更新缓存
	storeCacheLock.Lock()
	cachedStoreCountdown = countdown
	storeCacheExpiresAt = time.Now().Add(storeCacheDuration)
	storeCacheLock.Unlock()

	return countdown, nil
}

// ForceRefreshStoreCountdown 强制刷新商店倒计时缓存
func ForceRefreshStoreCountdown() (*StoreCountdown, error) {
	storeCacheLock.Lock()
	cachedStoreCountdown = nil
	storeCacheExpiresAt = time.Time{}
	storeCacheLock.Unlock()

	return GetStoreCountdownFromAPI()
}
