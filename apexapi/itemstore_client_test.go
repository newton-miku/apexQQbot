package apexapi

import (
	"fmt"
	"testing"
)

func TestItemStoreClient(t *testing.T) {
	client := NewItemStoreClient()

	// 第一步：测试获取 nonce
	fmt.Println("=== 步骤1: 获取 nonce ===")
	nonce, err := client.getNonce()
	if err != nil {
		t.Fatalf("获取 nonce 失败: %v", err)
	}
	fmt.Printf("成功获取 nonce: %s\n", nonce)

	// 第二步：调用 API
	fmt.Println("\n=== 步骤2: 调用 API ===")
	resp, err := client.QueryNextEvent("")
	if err != nil {
		t.Fatalf("API 调用失败: %v", err)
	}

	fmt.Printf("错误码: %d\n", resp.ErrCode)
	fmt.Printf("截止时间: %s\n", resp.Options.Deadline)
	fmt.Printf("服务器时间戳: %d\n", resp.Options.Now)
	fmt.Printf("导入标题: %s\n", resp.Options.ImportedTitle)
	fmt.Printf("倒计时查询限制: %d\n", resp.Options.CountdownQueryLimit)
	fmt.Printf("正计时限制: %d\n", resp.Options.CountupLimit)

	if !resp.IsSuccess() {
		t.Fatalf("API 返回错误: err_code=%d", resp.ErrCode)
	}

	fmt.Println("\n=== 测试通过! ===")
}
