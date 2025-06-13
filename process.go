package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/newton-miku/apexQQbot/apexapi"
	"github.com/tencent-connect/botgo/dto"
	botlog "github.com/tencent-connect/botgo/log"
	"github.com/tencent-connect/botgo/openapi"
	"golang.org/x/oauth2"
)

// Processor is a struct to process message
type Processor struct {
	api       openapi.OpenAPI
	appID     string
	appSecret string
	token     oauth2.TokenSource
}

// ProcessChannelMessage is a function to process message
func (p Processor) ProcessChannelMessage(input string, data *dto.WSATMessageData) error {
	msg := generateDemoMessage(input, dto.Message(*data))
	if err := p.sendChannelReply(context.Background(), data.ChannelID, msg); err != nil {
		_ = p.sendChannelReply(context.Background(), data.GroupID, genErrMessage(dto.Message(*data), err))
	}
	return nil
}

// ProcessInlineSearch is a function to process inline search
func (p Processor) ProcessInlineSearch(interaction *dto.WSInteractionData) error {
	if interaction.Data.Type != dto.InteractionDataTypeChatSearch {
		return fmt.Errorf("interaction data type not chat search")
	}
	search := &dto.SearchInputResolved{}
	if err := json.Unmarshal(interaction.Data.Resolved, search); err != nil {
		log.Println(err)
		return err
	}
	if search.Keyword != "test" {
		return fmt.Errorf("resolved search key not allowed")
	}
	searchRsp := &dto.SearchRsp{
		Layouts: []dto.SearchLayout{
			{
				LayoutType: 0,
				ActionType: 0,
				Title:      "内联搜索",
				Records: []dto.SearchRecord{
					{
						Cover: "https://pub.idqqimg.com/pc/misc/files/20211208/311cfc87ce394c62b7c9f0508658cf25.png",
						Title: "内联搜索标题",
						Tips:  "内联搜索 tips",
						URL:   "https://www.qq.com",
					},
				},
			},
		},
	}
	body, _ := json.Marshal(searchRsp)
	if err := p.api.PutInteraction(context.Background(), interaction.ID, string(body)); err != nil {
		log.Println("api call putInteractionInlineSearch  error: ", err)
		return err
	}
	return nil
}

func genErrMessage(data dto.Message, err error) *dto.MessageToCreate {
	return &dto.MessageToCreate{
		Timestamp: time.Now().UnixMilli(),
		Content:   fmt.Sprintf("处理异常:%v", err),
		MessageReference: &dto.MessageReference{
			// 引用这条消息
			MessageID:             data.ID,
			IgnoreGetMessageError: true,
		},
		MsgID: data.ID,
	}
}
func createMessage(data dto.Message, msg string) *dto.MessageToCreate {
	return &dto.MessageToCreate{
		Timestamp: time.Now().UnixMilli(),
		Content:   fmt.Sprint(msg),
		MessageReference: &dto.MessageReference{
			// 引用这条消息
			MessageID:             data.ID,
			IgnoreGetMessageError: true,
		},
		MsgID: data.ID,
	}
}
func createRichMessage(data dto.Message, msg string, msgseq ...int) *dto.MessageToCreate {
	msgSeq := uint32(1)
	if len(msgseq) > 0 {
		msgSeq = uint32(msgseq[0])
	}
	return &dto.MessageToCreate{
		Timestamp: time.Now().UnixMilli(),
		MsgType:   dto.RichMediaMsg,
		Content:   fmt.Sprint(msg),
		Media:     &dto.MediaInfo{},
		MsgID:     data.ID,
		MsgSeq:    msgSeq,
	}
}
func createImgMessage(data dto.Message, picContent []byte) *dto.RichMediaMessage {
	return &dto.RichMediaMessage{
		FileType:   1,
		SrvSendMsg: false,
		// Content:  fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(picContent)),
		URL:     "https://apexlegendsstatus.com/assets/maps/Olympus.png",
		Content: "测试",
	}
}

// ProcessGroupMessage 回复群消息
func (p Processor) ProcessGroupMessage(input string, data *dto.WSGroupATMessageData) error {
	// 提前 trim 输入
	input = strings.TrimSpace(input)
	// 定义命令关键词
	const (
		cmdPrefix = "/a"
	)
	var (
		mapCmds    = []string{"地图", "map"}
		playerCmds = []string{"查询", "player"}
		bindCmds   = []string{"绑定", "bind"}
		// helpCmds   = []string{"帮助", "help"}
	)

	// 工具函数：检查输入是否匹配任意命令
	isCommandMatch := func(input string, cmdLists ...[]string) bool {
		input = strings.TrimSpace(strings.ToLower(input))
		if strings.HasPrefix(input, strings.ToLower(cmdPrefix)) {
			input = input[len(cmdPrefix):]
		}
		for _, list := range cmdLists {
			for _, cmd := range list {
				if strings.HasPrefix(input, strings.ToLower(cmd)) {
					return true
				}
			}
		}
		return false
	}
	// 获取当前用户信息
	var qqUser *dto.User
	if data.Author != nil && data.Author.ID != "" {
		qqUser = data.Author
	}
	// 提前构造 message 对象
	msgBase := dto.Message(*data)

	// 统一处理绑定命令
	if isCommandMatch(input, bindCmds) {
		// 提取 EAID 并进行绑定操作
		EAID := ""
		// 检查是否有空格分隔符
		parts := strings.SplitN(input, " ", 2)
		if len(parts) > 1 {
			EAID = strings.TrimSpace(parts[1])
		}

		if EAID == "" {
			replyMsg := createMessage(msgBase, "请提供有效的 EAID，格式为 /a绑定 <EAID>")
			_ = p.sendGroupReply(context.Background(), data.GroupID, replyMsg)
			return nil
		}
		playerJson, err := apexapi.GetPlayerData(EAID)
		if err != nil {
			replyMsg := createMessage(msgBase, fmt.Sprint("绑定失败，查询信息时发送错误\n", err))
			_ = p.sendGroupReply(context.Background(), data.GroupID, replyMsg)
			return nil
		}
		var playerData map[string]interface{}
		err = json.Unmarshal([]byte(playerJson), &playerData)
		if err != nil {
			str := fmt.Sprintf("解析玩家数据出错: %v\n", err)
			replyMsg := createMessage(msgBase, fmt.Sprint("绑定失败，查询信息时发送错误\n", str))
			_ = p.sendGroupReply(context.Background(), data.GroupID, replyMsg)
			return nil
		}

		rankData, ok := playerData["rank"].(map[string]interface{})
		rankScore := 0
		if ok {
			rankScore = int(rankData["rankScore"].(float64))
		}

		// 更新绑定数据
		bindingData := apexapi.PlayerBindingData{
			QQ:             qqUser.ID,
			EAID:           EAID,
			LastUpdateTime: time.Now(),
			LastRankScore:  rankScore,
		}
		apexapi.Players.Set(qqUser.ID, bindingData)

		// 保存到文件
		if err := apexapi.Players.SaveBindingRecords(); err != nil {
			log.Printf("保存绑定记录失败：%v\n", err)
			replyMsg := createMessage(msgBase, fmt.Sprintf("保存绑定记录失败：%v", err))
			_ = p.sendGroupReply(context.Background(), data.GroupID, replyMsg)
			return nil
		}

		// 返回成功提示
		replyMsg := createMessage(msgBase, fmt.Sprintf("绑定成功！您的 EAID 是 %s", EAID))
		_ = p.sendGroupReply(context.Background(), data.GroupID, replyMsg)
		return nil
	}

	// 地图查询命令统一处理
	if isCommandMatch(input, mapCmds) {
		log.Println("处理地图查询命令")
		mapResultPath, err := apexapi.GetMapResult()
		if err != nil {
			log.Print("[ApexQueryMap] ", err)
			return err
		}

		// 读取图片字节数据
		qrContent, err := os.ReadFile(mapResultPath)
		if err != nil {
			botlog.Warnf("读取地图图片失败: %v", err)
			replyMsg := createMessage(msgBase, fmt.Sprintf("读取地图图片失败：%v", err))
			if sendErr := p.sendGroupReply(context.Background(), data.GroupID, replyMsg); sendErr != nil {
				log.Printf("发送错误消息失败: %v", sendErr)
			}
			return nil
		}

		imgRichMsg := createRichMessage(msgBase, "")
		err = p.sendGroupImgDataReply(context.Background(), data.GroupID, qrContent, imgRichMsg)
		if err != nil {
			botlog.Errorf("自建func发送地图图片失败: %v", err)
			return nil
		}

		log.Printf("发送地图图片成功")
		return nil
	}

	// 玩家查询命令
	if isCommandMatch(input, playerCmds) {
		EAID := ""
		bind := false
		// 检查是否有空格分隔符
		parts := strings.SplitN(input, " ", 2)
		if len(parts) > 1 {
			EAID = strings.TrimSpace(parts[1])
		}
		log.Println("处理玩家查询命令,EAID=", EAID)
		// 如果没有输入 EAID，尝试从绑定中获取
		if EAID == "" {
			bindingData, exists := apexapi.Players.Get(qqUser.ID)
			if !exists {
				errMsg := "您尚未绑定 EAID，请使用 /a绑定 <EAID> 进行绑定"
				replyMsg := createMessage(msgBase, errMsg)
				if sendErr := p.sendGroupReply(context.Background(), data.GroupID, replyMsg); sendErr != nil {
					log.Printf("发送绑定提示失败: %v", sendErr)
				}
				return nil
			}
			EAID = bindingData.EAID
			bind = true
		}

		dataStr, err := apexapi.GetPlayerData(EAID)
		if err != nil {
			_ = p.sendGroupReply(context.Background(), data.GroupID, genErrMessage(msgBase, err))
			return nil
		}

		// 增加对 dataStr 的有效性检查
		if len(dataStr) == 0 {
			_ = p.sendGroupReply(context.Background(), data.GroupID, genErrMessage(msgBase, fmt.Errorf("获取到空的玩家数据")))
			return nil
		}

		msg := apexapi.DisplayPlayerData(dataStr)
		if bind {
			rank, _ := apexapi.GetPlayerRank(dataStr)
			bindingData, _ := apexapi.Players.Get(qqUser.ID)
			bindingData.LastRankScore = rank
			bindingData.LastUpdateTime = time.Now()
			apexapi.Players.Set(qqUser.ID, bindingData)
		}
		replyMsg := createMessage(msgBase, msg)

		if err := p.sendGroupReply(context.Background(), data.GroupID, replyMsg); err != nil {
			log.Printf("发送回复消息失败: %v", err)
			return err
		}
		return nil
	}

	// 其他指令或默认行为
	msg := generateDemoMessage(input, msgBase)
	if err := p.sendGroupReply(context.Background(), data.GroupID, msg); err != nil {
		log.Printf("发送默认回复失败: %v", err)
		_ = p.sendGroupReply(context.Background(), data.GroupID, genErrMessage(msgBase, err))
	}

	return nil
}

// ProcessC2CMessage 回复C2C消息
func (p Processor) ProcessC2CMessage(input string, data *dto.WSC2CMessageData) error {
	userID := ""
	if data.Author != nil && data.Author.ID != "" {
		userID = data.Author.ID
	}
	msg := generateDemoMessage(input, dto.Message(*data))
	if err := p.sendC2CReply(context.Background(), userID, msg); err != nil {
		_ = p.sendC2CReply(context.Background(), userID, genErrMessage(dto.Message(*data), err))
	}
	return nil
}

func generateDemoMessage(input string, data dto.Message) *dto.MessageToCreate {
	log.Printf("收到指令: %+v", input)
	msg := ""
	if len(input) > 0 {
		msg += "收到:" + input
	}
	for _, _v := range data.Attachments {
		msg += ",收到文件类型:" + _v.ContentType
	}
	return &dto.MessageToCreate{
		Timestamp: time.Now().UnixMilli(),
		Content:   msg,
		MessageReference: &dto.MessageReference{
			// 引用这条消息
			MessageID:             data.ID,
			IgnoreGetMessageError: true,
		},
		MsgID: data.ID,
	}
}

// ProcessFriend 处理 c2c 好友事件
func (p Processor) ProcessFriend(wsEventType string, data *dto.WSC2CFriendData) error {
	// 请注意，这里是主动推送添加好友事件，后续改为 event id 被动消息
	replyMsg := dto.MessageToCreate{
		Timestamp: time.Now().UnixMilli(),
		Content:   "",
	}
	var content string
	switch strings.ToLower(wsEventType) {
	case strings.ToLower(string(dto.EventC2CFriendAdd)):
		log.Println("添加好友")
		content = fmt.Sprintf("ID为 %s 的用户添加机器人为好友", data.OpenID)
	case strings.ToLower(string(dto.EventC2CFriendDel)):
		log.Println("删除好友")
	default:
		log.Println(wsEventType)
		return nil
	}
	replyMsg.Content = content
	_, err := p.api.PostC2CMessage(
		context.Background(),
		data.OpenID,
		replyMsg,
	)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
