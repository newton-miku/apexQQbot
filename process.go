package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
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
	m := &dto.MessageToCreate{
		Timestamp: time.Now().UnixMilli(),
		MsgType:   dto.RichMediaMsg,
		Content:   fmt.Sprint(msg),
		Media:     &dto.MediaInfo{},
		MsgID:     data.ID,
		MsgSeq:    msgSeq,
	}
	return m
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

// 定义命令关键词
const (
	cmdPrefix = "/a"
)

// 工具函数：检查输入是否匹配任意命令
func isCommandMatch(input string, cmdLists ...[]string) bool {
	input = strings.TrimSpace(strings.ToLower(input))
	if strings.HasPrefix(input, strings.ToLower(cmdPrefix)) {
		input = input[len(cmdPrefix):]
		// 修复：去除前缀后的空格，确保命令能正确匹配
		input = strings.TrimSpace(input)
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

func parseEAIDFromInput(input string) string {
	parts := strings.SplitN(input, " ", 2)
	if len(parts) > 1 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
func normalizeInput(input string) string {
	s := strings.TrimSpace(strings.ToLower(input))
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return s
}
func sendText(p Processor, isGroup bool, groupID string, userID string, msg dto.APIMessage) error {
	if isGroup {
		return p.sendGroupReply(context.Background(), groupID, msg)
	}
	return p.sendC2CReply(context.Background(), userID, msg)
}
func handleBind(p Processor, qqUser *dto.User, EAID string, base dto.Message, isGroup bool, groupID string, userID string) error {
	if EAID == "" {
		if isGroup {
			return sendText(p, true, groupID, "", createMessage(base, "请提供有效的 EAID（必须为EA平台中的用户名，不可使用Steam名称）\n格式为 /a绑定 <EAID>\n例如如：/a绑定 MDY_KaLe"))
		}
		return sendText(p, false, "", userID, createMessage(base, "请提供有效的 EAID，格式为 /a绑定 <EAID>"))
	}
	playerJson, err := apexapi.GetPlayerData(EAID)
	if err != nil {
		return sendText(p, isGroup, groupID, userID, createMessage(base, fmt.Sprint("绑定失败，查询信息时发送错误\n", err)))
	}
	var playerData map[string]interface{}
	if err := json.Unmarshal([]byte(playerJson), &playerData); err != nil {
		return sendText(p, isGroup, groupID, userID, createMessage(base, fmt.Sprint("绑定失败，查询信息时发送错误，请稍后再试\n", fmt.Sprintf("解析玩家数据出错: %v\n", err))))
	}
	rankScore := 0
	if g, ok := playerData["global"].(map[string]interface{}); ok {
		if r, ok := g["rank"].(map[string]interface{}); ok {
			if v, ok := r["rankScore"].(float64); ok {
				rankScore = int(v)
			}
		}
	}
	if rankScore == 0 {
		if r, ok := playerData["rank"].(map[string]interface{}); ok {
			if v, ok := r["rankScore"].(float64); ok {
				rankScore = int(v)
			}
		}
	}
	uid := ""
	if g, ok := playerData["global"].(map[string]interface{}); ok {
		if v, ok := g["uid"].(string); ok {
			uid = v
		}
	}
	bindingData := apexapi.PlayerBindingData{
		QQ:             qqUser.ID,
		EAID:           EAID,
		EAUID:          uid,
		LastUpdateTime: time.Now(),
		LastRankScore:  rankScore,
	}
	apexapi.Players.Set(qqUser.ID, bindingData)
	if err := apexapi.Players.SaveBindingRecords(); err != nil {
		return sendText(p, isGroup, groupID, userID, createMessage(base, fmt.Sprintf("保存绑定记录失败：%v", err)))
	}
	return sendText(p, isGroup, groupID, userID, createMessage(base, fmt.Sprintf("绑定成功！您的 EAID 是 %s", EAID)))
}
func requireEAIDOrBinding(userID string, EAID string) (string, bool, int, time.Time, bool) {
	if EAID != "" {
		return EAID, false, 0, time.Time{}, true
	}
	bindingData, exists := apexapi.Players.Get(userID)
	if !exists {
		return "", false, 0, time.Time{}, false
	}
	return bindingData.EAID, true, bindingData.LastRankScore, bindingData.LastUpdateTime, true
}
func handlePlayerQuery(p Processor, qqUser *dto.User, EAID string, base dto.Message, isGroup bool, groupID string, userID string) error {
	eaid, bind, lastScore, lastUpdateTime, ok := requireEAIDOrBinding(qqUser.ID, EAID)
	if !ok {
		if isGroup {
			return sendText(p, true, groupID, "", createMessage(base, "您尚未绑定 EAID，请使用 /a绑定 <EAID> 进行绑定"))
		}
		return sendText(p, false, "", userID, createMessage(base, "您尚未绑定 EAID，请使用 /a绑定 <EAID> 进行绑定"))
	}
	dataStr, err := apexapi.GetPlayerData(eaid)
	if err != nil {
		return sendText(p, isGroup, groupID, userID, genErrMessage(base, err))
	}
	if len(dataStr) == 0 {
		return sendText(p, isGroup, groupID, userID, genErrMessage(base, fmt.Errorf("获取到空的玩家数据")))
	}
	var msg string
	if bind {
		msg = apexapi.DisplayPlayerData(dataStr, apexapi.DisplayChangedOption{
			LastScore: lastScore,
			LastTime:  lastUpdateTime,
		})
	} else {
		msg = apexapi.DisplayPlayerData(dataStr)
	}
	if bind {
		if rank, _ := apexapi.GetPlayerRank(dataStr); rank > 0 {
			bindingData, _ := apexapi.Players.Get(qqUser.ID)
			bindingData.LastRankScore = rank
			bindingData.LastUpdateTime = time.Now()
			apexapi.Players.Set(qqUser.ID, bindingData)
		}
	}
	return sendText(p, isGroup, groupID, userID, createMessage(base, msg))
}
func helpMessage() string {
	var b strings.Builder
	b.WriteString("以下为指令示例（其中[]中的表示可选项）：\n")
	b.WriteString("获取当前轮换地图：@机器人 [/a]地图\n")
	b.WriteString("绑定/换绑EA账号：@机器人 [/a]绑定 EAID,如@机器人 绑定 kasaa\n")
	b.WriteString("查询绑定的EA账号数据：@机器人 [/a]查询\n")
	b.WriteString("获取区服对应中英文对照：@机器人 [/a]区服\n")
	return b.String()
}

// ProcessGroupMessage 回复群消息
func (p Processor) ProcessGroupMessage(input string, data *dto.WSGroupATMessageData) error {
	input = normalizeInput(input)
	var (
		mapCmds    = []string{"地图", "map"}
		playerCmds = []string{"查询", "player"}
		bindCmds   = []string{"绑定", "bind"}
		serverCmds = []string{"区服", "server"}
		helpCmds   = []string{"帮助", "help"}
	)
	var qqUser *dto.User
	if data.Author != nil && data.Author.ID != "" {
		qqUser = data.Author
	}
	msgBase := dto.Message(*data)

	if isCommandMatch(input, bindCmds) {
		EAID := parseEAIDFromInput(input)
		_ = handleBind(p, qqUser, EAID, msgBase, true, data.GroupID, "")
		return nil
	}

	if isCommandMatch(input, mapCmds) {
		log.Println("处理地图查询命令")
		mapResultPath, err := apexapi.GetMapResult()
		if err != nil {
			log.Print("[ApexQueryMap] ", err)
			return err
		}

		err, shouldReturn := p.GetImgAndSendToGroup(mapResultPath, msgBase, data)
		if shouldReturn {
			return err
		}

		log.Printf("发送地图图片成功")
		return nil
	}
	if isCommandMatch(input, serverCmds) {
		log.Println("处理区服查询命令")

		err, shouldReturn := p.GetImgAndSendToGroup("asset/Static/Server.png", msgBase, data)
		if shouldReturn {
			return err
		}

		log.Printf("发送服务器图片成功")
		return nil
	}

	if isCommandMatch(input, playerCmds) {
		EAID := parseEAIDFromInput(input)
		_ = handlePlayerQuery(p, qqUser, EAID, msgBase, true, data.GroupID, "")
		return nil
	}
	if isCommandMatch(input, helpCmds) {
		replyMsg := createMessage(msgBase, helpMessage())
		_ = p.sendGroupReply(context.Background(), data.GroupID, replyMsg)
		return nil
	}

	msg := generateDemoMessage(input, msgBase)
	if err := p.sendGroupReply(context.Background(), data.GroupID, msg); err != nil {
		log.Printf("发送默认回复失败: %v", err)
		_ = p.sendGroupReply(context.Background(), data.GroupID, genErrMessage(msgBase, err))
	}

	return nil
}

func (p Processor) GetImgAndSendToGroup(mapResultPath string, msgBase dto.Message, data *dto.WSGroupATMessageData) (error, bool) {
	qrContent, err := os.ReadFile(mapResultPath)
	if err != nil {
		botlog.Warnf("读取地图图片失败: %v", err)
		replyMsg := createMessage(msgBase, "读取地图图片失败，请反馈至开发人员")
		if sendErr := p.sendGroupReply(context.Background(), data.GroupID, replyMsg); sendErr != nil {
			log.Printf("发送错误消息失败: %v", sendErr)
		}
		return nil, true
	}

	imgRichMsg := createRichMessage(msgBase, "")
	err = p.sendGroupImgDataReply(context.Background(), data.GroupID, qrContent, imgRichMsg)
	if err != nil {
		botlog.Errorf("发送地图图片失败: %v", err)
		return nil, true
	}
	return nil, false
}

// ProcessC2CMessage 回复C2C消息
func (p Processor) ProcessC2CMessage(input string, data *dto.WSC2CMessageData) error {
	input = normalizeInput(input)
	var qqUser *dto.User
	userID := ""
	if data.Author != nil && data.Author.ID != "" {
		userID = data.Author.ID
		qqUser = data.Author
	}
	var (
		playerCmds = []string{"查询", "player"}
		bindCmds   = []string{"绑定", "bind"}
	)

	msgBase := dto.Message(*data)

	if isCommandMatch(input, bindCmds) {
		EAID := parseEAIDFromInput(input)
		_ = handleBind(p, qqUser, EAID, msgBase, false, "", userID)
		return nil
	}

	if isCommandMatch(input, playerCmds) {
		EAID := parseEAIDFromInput(input)
		_ = handlePlayerQuery(p, qqUser, EAID, msgBase, false, "", userID)
		return nil
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
		msg += "您输入的指令指令\"" + input + "\"似乎有误，请使用帮助获取指令手册"
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
