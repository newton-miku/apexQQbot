package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Translator 翻译器结构体，管理翻译映射、替换器和文件监听
type Translator struct {
	mu         sync.RWMutex      // 读写锁，保证并发安全（读多写少场景最优）
	transMap   map[string]string // 存储关键字映射（如：key -> 翻译值）
	replacer   *strings.Replacer // 预编译的替换器，提升替换效率
	configPath string            // 配置文件路径
	watcher    *fsnotify.Watcher // 文件监听器
}

// NewTranslator 创建翻译器实例，初始化并启动文件监听
func NewTranslator(configPath string) (*Translator, error) {
	// 初始化文件监听器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("创建文件监听器失败：%w", err)
	}

	t := &Translator{
		configPath: configPath,
		watcher:    watcher,
		transMap:   make(map[string]string),
	}

	// 第一次加载配置
	if err := t.loadConfig(); err != nil {
		return nil, fmt.Errorf("加载配置文件失败：%w", err)
	}

	// 启动goroutine监听文件变化（非阻塞，不影响主程序）
	go t.watchConfig()

	return t, nil
}

// loadConfig 从文件加载配置，更新transMap和replacer
func (t *Translator) loadConfig() error {
	// 读取配置文件内容
	data, err := os.ReadFile(t.configPath)
	if err != nil {
		return fmt.Errorf("读取文件失败：%w", err)
	}

	// 临时map，避免直接修改原map导致并发问题
	tempMap := make(map[string]string)
	if err := json.Unmarshal(data, &tempMap); err != nil {
		return fmt.Errorf("解析JSON失败：%w", err)
	}

	// 加写锁，更新数据
	t.mu.Lock()
	defer t.mu.Unlock()

	// 更新翻译映射
	t.transMap = tempMap

	// 构建strings.Replacer的参数（格式：key1, value1, key2, value2...）
	replacerArgs := make([]string, 0, len(tempMap)*2)
	for k, v := range tempMap {
		replacerArgs = append(replacerArgs, k, v)
	}
	// 预编译替换器
	t.replacer = strings.NewReplacer(replacerArgs...)

	fmt.Println("翻译器 配置文件加载成功，当前映射：", t.transMap)
	return nil
}

// watchConfig 监听配置文件变化，触发热重载
func (t *Translator) watchConfig() {
	// 添加要监听的文件
	if err := t.watcher.Add(t.configPath); err != nil {
		fmt.Printf("翻译器 监听文件失败：%v\n", err)
		return
	}

	// 循环监听事件
	for {
		select {
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}
			// 只处理文件修改或写入完成的事件（避免重复触发）
			if event.Op&fsnotify.Write == fsnotify.Write {
				fmt.Println("\n翻译器 检测到配置文件变化，正在重载...")
				// 延迟加载（避免文件还没写完就触发重载，导致解析失败）
				time.Sleep(100 * time.Millisecond)
				if err := t.loadConfig(); err != nil {
					fmt.Printf("翻译器 配置文件重载失败：%v\n", err)
				}
			}
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("翻译器 文件监听错误：%v\n", err)
		}
	}
}

// Translate 执行文字替换（核心功能）
func (t *Translator) Translate(input string) string {
	// 加读锁（允许多个协程同时读，提高并发性能）
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.replacer == nil {
		return input // 替换器未初始化时，返回原字符串
	}
	// 使用预编译的替换器执行替换
	return t.replacer.Replace(input)
}

// Close 关闭监听器（资源释放）
func (t *Translator) Close() error {
	return t.watcher.Close()
}

// 测试主函数
// func sample() {
// 	// 配置文件路径（你可以根据实际情况修改）
// 	configPath := "translation.json"

// 	// 创建翻译器
// 	translator, err := NewTranslator(configPath)
// 	if err != nil {
// 		fmt.Printf("初始化翻译器失败：%v\n", err)
// 		return
// 	}
// 	defer translator.Close()

// 	// 模拟持续运行，接收输入并翻译
// 	fmt.Println("\n翻译器已启动，输入文字进行翻译（输入exit退出）：")
// 	var input string
// 	for {
// 		fmt.Print("> ")
// 		_, err := fmt.Scanln(&input)
// 		if err != nil {
// 			// 处理输入换行等问题
// 			input = ""
// 			continue
// 		}
// 		if input == "exit" {
// 			fmt.Println("退出程序...")
// 			break
// 		}
// 		// 执行翻译
// 		result := translator.Translate(input)
// 		fmt.Println("翻译结果：", result)
// 	}
// }
