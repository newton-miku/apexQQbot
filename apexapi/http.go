package apexapi

import (
	"net/http"
	"sync"
	"time"
)

// ============ HTTP Client 复用 ============

var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// GetHTTPClient 获取复用的 HTTP Client
func GetHTTPClient(timeout time.Duration) *http.Client {
	httpClientOnce.Do(func() {
		httpClient = &http.Client{
			Timeout: timeout,
		}
	})
	return httpClient
}
