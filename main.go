package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/newton-miku/apexQQbot/apexapi"
	"github.com/newton-miku/apexQQbot/tools"
	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/dto/message"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/interaction/webhook"
	"github.com/tencent-connect/botgo/token"
)

const (
	host_ = "0.0.0.0"
	port_ = 9000
	path_ = "/qqbot"
)

var (
	DebugFlag   = false
	VersionFlag = false
)

func init() {
	flag.BoolVar(&DebugFlag, "debug", false, "enable debug mode")
	flag.BoolVar(&VersionFlag, "v", false, "output version information and exit")
	flag.Parse()
}

// 消息处理器，持有 openapi 对象
var processor Processor

const configFile = "conf/config.yaml"

func main() {
	if VersionFlag {
		tools.PrintVersion()
	}

	// 根据 DebugFlag 设置日志级别
	logLevel := InfoLevel
	if DebugFlag {
		logLevel = DebugLevel
	}

	// 初始化日志
	logger, err := New("./", logLevel)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v", err)
		os.Exit(1)
	}
	botgo.SetLogger(logger)
	defer logger.Close()

	logger.Infof("apexQQbot Version: %s", tools.Version)

	// 初始化 apexapi 模块
	apexapi.Init()

	// 获取配置
	config := apexapi.GetAppConfig()
	if config.AppID == "" || config.AppID == "your_app_id" {
		logger.Fatal("配置无效: 请在配置文件中设置正确的 AppID")
	}

	credentials := &token.QQBotCredentials{
		AppID:     config.AppID,
		AppSecret: config.AppSecret,
	}

	tokenSource := token.NewQQBotTokenSource(credentials)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := token.StartRefreshAccessToken(ctx, tokenSource); err != nil {
		logger.Fatalf("刷新 Token 失败: %v", err)
	}

	logger.Info("准备初始化 openapi")
	api := botgo.NewOpenAPI(credentials.AppID, tokenSource).WithTimeout(5 * time.Second).SetDebug(DebugFlag)
	processor = Processor{
		api:       api,
		appID:     credentials.AppID,
		appSecret: credentials.AppSecret,
		token:     tokenSource,
	}

	// 注册处理函数
	_ = event.RegisterHandlers(
		GroupATMessageEventHandler(),
		C2CMessageEventHandler(),
		ChannelATMessageEventHandler(),
	)

	http.HandleFunc(path_, func(writer http.ResponseWriter, request *http.Request) {
		webhook.HTTPHandler(writer, request, credentials)
	})

	logger.Info("准备启动 http server")
	if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host_, port_), nil); err != nil {
		logger.Fatalf("启动服务器失败: %v", err)
	}
}

// ============ 事件处理器 ============

// ChannelATMessageEventHandler 处理频道 at 消息
func ChannelATMessageEventHandler() event.ATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSATMessageData) error {
		input := strings.ToLower(message.ETLInput(data.Content))
		return processor.ProcessChannelMessage(input, data)
	}
}

// InteractionHandler 处理内联交互事件
func InteractionHandler() event.InteractionEventHandler {
	return func(event *dto.WSPayload, data *dto.WSInteractionData) error {
		fmt.Println(data)
		return processor.ProcessInlineSearch(data)
	}
}

// GroupATMessageEventHandler 处理群 at 消息
func GroupATMessageEventHandler() event.GroupATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGroupATMessageData) error {
		input := strings.ToLower(message.ETLInput(data.Content))
		return processor.ProcessGroupMessage(input, data)
	}
}

// C2CMessageEventHandler 处理 C2C 消息
func C2CMessageEventHandler() event.C2CMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSC2CMessageData) error {
		input := data.Content
		return processor.ProcessC2CMessage(input, data)
	}
}

// C2CFriendEventHandler 处理好友关系变更
func C2CFriendEventHandler() event.C2CFriendEventHandler {
	return func(event *dto.WSPayload, data *dto.WSC2CFriendData) error {
		fmt.Println(data)
		return processor.ProcessFriend(string(event.Type), data)
	}
}

// GuildEventHandler 处理频道事件
func GuildEventHandler() event.GuildEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGuildData) error {
		fmt.Println(data)
		return nil
	}
}

// ChannelEventHandler 处理子频道事件
func ChannelEventHandler() event.ChannelEventHandler {
	return func(event *dto.WSPayload, data *dto.WSChannelData) error {
		fmt.Println(data)
		return nil
	}
}

// GuildMemberEventHandler 处理成员变更
func GuildMemberEventHandler() event.GuildMemberEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGuildMemberData) error {
		fmt.Println(data)
		return nil
	}
}

// GuildDirectMessageHandler 处理私信事件
func GuildDirectMessageHandler() event.DirectMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSDirectMessageData) error {
		fmt.Println(data)
		return nil
	}
}

// GuildMessageHandler 处理消息事件
func GuildMessageHandler() event.MessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSMessageData) error {
		fmt.Println(data)
		return nil
	}
}
