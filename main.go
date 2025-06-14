package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	"gopkg.in/yaml.v3"
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
	log.Printf("apexQQbot Version: %s", tools.Version)
	basePath, _ := os.Getwd()
	logger, err := New("./", DebugLevel)
	// 把新的 logger 设置到 sdk 上，替换掉老的控制台 logger
	botgo.SetLogger(logger)
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
		return
	}

	confPath := filepath.Join(basePath, configFile)
	log.Printf("Loading config from: %s", confPath)
	//  初始化 apexapi
	apexapi.Init()

	// 加载 appid 和 token
	content, err := os.ReadFile(confPath)
	if err != nil {
		log.Fatalln("load config file failed, err:", err)
	}
	credentials := &token.QQBotCredentials{
		AppID:     "",
		AppSecret: "",
	}
	if err = yaml.Unmarshal(content, &credentials); err != nil {
		log.Fatalln("parse config failed, err:", err)
	}
	tokenSource := token.NewQQBotTokenSource(credentials)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() //释放刷新协程
	if err = token.StartRefreshAccessToken(ctx, tokenSource); err != nil {
		log.Fatalln(err)
	}
	// 初始化 openapi，正式环境
	api := botgo.NewOpenAPI(credentials.AppID, tokenSource).WithTimeout(5 * time.Second).SetDebug(DebugFlag)
	processor = Processor{
		api:       api,
		appID:     credentials.AppID,
		appSecret: credentials.AppSecret,
		token:     tokenSource,
	}
	// 注册处理函数
	_ = event.RegisterHandlers(
		// ***********消息事件***********
		// 群@机器人消息事件
		GroupATMessageEventHandler(),
		// C2C消息事件
		C2CMessageEventHandler(),
		// 频道@机器人事件
		ChannelATMessageEventHandler(),
	)
	http.HandleFunc(path_, func(writer http.ResponseWriter, request *http.Request) {
		webhook.HTTPHandler(writer, request, credentials)
	})
	if err = http.ListenAndServe(fmt.Sprintf("%s:%d", host_, port_), nil); err != nil {
		log.Fatal("setup server fatal:", err)
	}
}

// ChannelATMessageEventHandler 实现处理 at 消息的回调
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

// GroupATMessageEventHandler 实现处理 at 消息的回调
func GroupATMessageEventHandler() event.GroupATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGroupATMessageData) error {
		input := strings.ToLower(message.ETLInput(data.Content))
		return processor.ProcessGroupMessage(input, data)
	}
}

// C2CMessageEventHandler 实现处理 at 消息的回调
func C2CMessageEventHandler() event.C2CMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSC2CMessageData) error {
		input := data.Content
		return processor.ProcessC2CMessage(input, data)
	}
}

// C2CFriendEventHandler 实现处理好友关系变更的回调
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

// GuildMemberEventHandler 处理成员变更事件
func GuildMemberEventHandler() event.GuildMemberEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGuildMemberData) error {
		fmt.Println(data)
		return nil
	}
}

// GuildDirectMessageHandler 处理频道私信事件
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
