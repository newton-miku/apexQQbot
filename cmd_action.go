package apexQQbot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tencent-connect/botgo/dto"
	botlog "github.com/tencent-connect/botgo/log"
)

type ImgRichData struct {
	Group_openid string `json:"group_openid"`
	File_type    int    `json:"file_type"`
	File_data    string `json:"file_data"`
	Srv_send_msg bool   `json:"srv_send_msg"`
}

func (p Processor) setEmoji(ctx context.Context, channelID string, messageID string) {
	err := p.api.CreateMessageReaction(
		ctx, channelID, messageID, dto.Emoji{
			ID:   "307",
			Type: 1,
		},
	)
	if err != nil {
		log.Println(err)
	}
}

func (p Processor) setPins(ctx context.Context, channelID, msgID string) {
	_, err := p.api.AddPins(ctx, channelID, msgID)
	if err != nil {
		log.Println(err)
	}
}

func (p Processor) setAnnounces(ctx context.Context, data *dto.WSATMessageData) {
	if _, err := p.api.CreateChannelAnnounces(
		ctx, data.ChannelID,
		&dto.ChannelAnnouncesToCreate{MessageID: data.ID},
	); err != nil {
		log.Println(err)
	}
}

func (p Processor) sendChannelReply(ctx context.Context, channelID string, toCreate *dto.MessageToCreate) error {
	if _, err := p.api.PostMessage(ctx, channelID, toCreate); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (p Processor) sendGroupReply(ctx context.Context, groupID string, toCreate dto.APIMessage) error {
	log.Printf("EVENT ID:%v", toCreate.GetEventID())
	if _, err := p.api.PostGroupMessage(ctx, groupID, toCreate); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (p Processor) SendGroupFileByBase64(ctx context.Context, groupOpenID string, fileType int, fileData []byte, srvSendMsg bool) (*dto.MediaInfo, error) {

	// 构建请求体
	payload := ImgRichData{
		Group_openid: groupOpenID,
		File_type:    fileType,
		File_data:    base64.StdEncoding.EncodeToString(fileData),
		Srv_send_msg: srvSendMsg,
	}

	media, err := p.UploadPicToGroup(ctx, groupOpenID, payload)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	log.Println("bin MediaInfo:", string(media.FileInfo))
	botlog.Debug("bin MediaInfo:", string(media.FileInfo))

	return media, err
}

// base64上传图片到群
func (p Processor) UploadPicToGroup(ctx context.Context, groupID string, body ImgRichData) (*dto.MediaInfo, error) {
	url := fmt.Sprintf("https://api.sgroup.qq.com/v2/groups/%s/files", groupID)
	fileInfo, err := p.api.Transport(ctx, "POST", url, body)
	if err != nil {
		botlog.Errorf("Transport request failed: %v", err)
		return nil, err
	}
	if len(fileInfo) == 0 {
		botlog.Errorf("UploadPicToGroup: received empty response body")
		return nil, fmt.Errorf("received empty response body")
	}

	var media dto.MediaInfo
	err = json.Unmarshal(fileInfo, &media)
	if err != nil {
		botlog.Errorf("JSON 解析失败: %v", err)
		return nil, err
	}
	return &media, nil
}

// 通过resty手动实现上传图片
func (p Processor) SendPicToDirectMsg(ctx context.Context, group_openid string, data map[string]interface{}) (*dto.MediaInfo, error) {
	tk, _ := p.token.Token()
	payload, err := json.Marshal(data)
	if err != nil {
		botlog.Errorf("Failed to marshal payload: %v", err)
		return nil, err
	}
	resp, err := resty.New().R().
		SetContext(ctx).
		SetAuthScheme("Bot").
		SetAuthScheme(tk.TokenType).
		SetAuthToken(tk.AccessToken).
		SetBody(payload).
		SetContentLength(true).
		SetResult(dto.MediaInfo{}).
		SetPathParam("group_openid", group_openid).
		Post(fmt.Sprintf("%s://%s%s", "https", "api.sgroup.qq.com", "/v2/groups/{group_openid}/files"))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.MediaInfo), nil
}

// 通过本地方式上传图片（Base64）
func (p Processor) sendGroupImgDataReply(ctx context.Context, groupID string, fileData []byte, toSend dto.APIMessage) error {
	// 构建请求体
	payload := ImgRichData{
		Group_openid: groupID,
		File_type:    1,
		File_data:    base64.StdEncoding.EncodeToString(fileData),
		Srv_send_msg: false,
	}
	fileInfo, err := p.UploadPicToGroup(ctx, groupID, payload)
	if err != nil {
		log.Println("[sendGroupImgDataReply]  UploadPicToGroup Err:", err, " fileInfo:", fileInfo)
		botlog.Debug("UploadPicToGroup Err:", err, " fileInfo:", fileInfo)
		return err
	}
	botlog.Debug("bin MediaInfo:")
	toSend.(*dto.MessageToCreate).Media = fileInfo
	toSend.(*dto.MessageToCreate).Timestamp = time.Now().UnixMilli()
	_, err = p.api.PostGroupMessage(ctx, groupID, toSend)
	if err != nil {
		if strings.Contains(err.Error(), "消息被去重，请检查请求msgseq") {
			return nil
		}
		return err
	}
	return nil
}

// 通过URL方式发送图片
func (p Processor) sendGroupImgReply(ctx context.Context, groupID string, toCreate dto.APIMessage, toSend dto.APIMessage) error {
	fileInfo, err := p.api.PostGroupMessage(ctx, groupID, toCreate)
	if err != nil {
		log.Println(err)
		return err
	}
	toSend.(*dto.MessageToCreate).Media.FileInfo = fileInfo.FileInfo
	_, err = p.api.PostGroupMessage(ctx, groupID, toSend)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (p Processor) sendC2CReply(ctx context.Context, userID string, toCreate dto.APIMessage) error {
	log.Printf("EVENT ID:%v", toCreate.GetEventID())
	if _, err := p.api.PostC2CMessage(ctx, userID, toCreate); err != nil {
		if strings.Contains(err.Error(), "消息被去重，请检查请求msgseq") {
			return nil
		}
		log.Println(err)
		return err
	}
	return nil
}
