package wechat

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"wechat-mp-gpt/chatgpt"
	"wechat-mp-gpt/config"
	"wechat-mp-gpt/entity"
	"wechat-mp-gpt/mj"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
)

type MsgHandler interface {
	Handle(msg *WxMsg, c *gin.Context)
}

type EventHandler struct {
}

type TextHandler struct {
}

type DefaultHandler struct {
}

type WxRspMsg struct {
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	XMLName      xml.Name `xml:"xml"`
}

type WxMsg struct {
	ToUserName   string `xml:"ToUserName"`
	FromUserName string `xml:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime"`
	MsgType      string `xml:"MsgType"`
	Content      string `xml:"Content"`
	MsgId        string `xml:"MsgId"`
	MsgDataId    string `xml:"MsgDataId"`
	Idx          string `xml:"Idx"`
}

type WxCustomerTextMsg struct {
	Touser  string             `json:"touser"`
	MsgType string             `json:"msgtype"`
	Text    WxCustomerTextItem `json:"text"`
}

type WxCustomerImageMsg struct {
	Touser  string              `json:"touser"`
	MsgType string              `json:"msgtype"`
	Image   WxCustomerImageItem `json:"image"`
}

type WxCustomerTextItem struct {
	Content string `json:"content"`
}

type WxCustomerImageItem struct {
	MediaId string `json:"media_id"`
}

type WxMediaRsp struct {
	Type     string `json:"type"`
	MediaID  string `json:"media_id"`
	CreateAt int16  `json:"create_at"`
}

// 微信返回文本大小限制
const contentLimitSize = 2000

var (
	reqGroup singleflight.Group
)

// 事件回调处理
func ProcessEvent(msg *WxMsg, c *gin.Context) {
	var handler MsgHandler
	switch msg.MsgType {
	case "event":
		handler = &EventHandler{}
	case "text":
		handler = &TextHandler{}
	default:
		handler = &DefaultHandler{}
	}
	handler.Handle(msg, c)
}

// 事件消息处理器
func (handler *EventHandler) Handle(msg *WxMsg, c *gin.Context) {
	rspMsg := &WxRspMsg{
		FromUserName: msg.ToUserName,
		ToUserName:   msg.FromUserName,
		CreateTime:   time.Now().Unix(),
		MsgType:      "text",
		Content:      "感谢关注",
	}

	result, _ := xml.Marshal(rspMsg)
	c.Writer.Write(result)
}

// 文本消息处理器
func (handler *TextHandler) Handle(msg *WxMsg, c *gin.Context) {
	rspMsg := &WxRspMsg{
		FromUserName: msg.ToUserName,
		ToUserName:   msg.FromUserName,
		CreateTime:   time.Now().Unix(),
		MsgType:      "text",
		Content:      "",
	}

	// 白名单功能是否提供出去
	// 不在白名单返回success
	if !config.IsWhiteList(msg.FromUserName) {
		c.Writer.Write([]byte("success"))
		return
	}

	defaultRespone := ""
	if strings.HasPrefix(msg.Content, "@image ") {
		msg.Content = strings.ReplaceAll(msg.Content, "@image ", "")
		// gpt生成图片
		go func() {
			imageUrl := chatgpt.DefaultGPT().GenerateImage(msg.Content)
			SendImage(msg, imageUrl)
		}()
		c.Writer.Write([]byte(defaultRespone))
		return
	} else if strings.HasPrefix(msg.Content, "@mj ") {
		msg.Content = strings.ReplaceAll(msg.Content, "@mj ", "")
		go func() {
			data, err := json.Marshal(msg)
			if err != nil {
				log.Error("序列化错误 err=%v\n", err)
				return
			}
			mj.DefaultMJ().Imagine(msg.Content, string(data))
		}()
		c.Writer.Write([]byte(defaultRespone))
		return
	} else {
		var ch <-chan entity.Result
		content, err, _ := reqGroup.Do(msg.MsgId, func() (interface{}, error) {
			ch = chatgpt.DefaultGPT().SendMsgChan(msg.FromUserName, msg.Content)
			select {
			case result := <-ch:
				return result.Data, errors.New("error")
			case <-time.After(14*time.Second + 300*time.Millisecond):
				// 超时返回错误
				log.Warn("chatgpt timeout")
				return "", errors.New("timeout")
			}
		})

		if err != nil {
			// TODO 重复了三次调用，是否存在资源问题
			if err.Error() == "timeout" {
				// 发送客服消息
				go SendCustomerMsg(ch, msg)
				c.Writer.Write([]byte(defaultRespone))
				return
			}
		}
		defaultRespone = content.(string)
	}

	rspMsg.Content = defaultRespone
	result, _ := xml.Marshal(rspMsg)

	log.Info("return msg: %v\n", *rspMsg)
	c.Writer.Write(result)
}

// 默认消息类型处理器
func (handler *DefaultHandler) Handle(msg *WxMsg, c *gin.Context) {
	log.Printf("type:[%s] not support", msg.MsgType)
	c.Writer.Write([]byte("success"))
}

// 发送客服消息
func SendCustomerMsg(result <-chan entity.Result, msg *WxMsg) {
	str := <-result
	if str.Data == "" {
		return
	}
	// 消息内容拆分
	res := msgSpilt(str.Data)
	for i := 0; i < len(res); i++ {
		SendCustomerMsgCore(res[i], msg, "text")
	}
}

// 发送客服消息核心
func SendCustomerMsgCore(content string, msg *WxMsg, msgType string) {
	token := DefaultWechat().token
	var build strings.Builder
	build.WriteString(customerUrlTemplate)
	build.WriteString(token)

	url := build.String()

	var data []byte
	if msgType == "text" {
		customerTextItem := &WxCustomerTextItem{
			Content: content,
		}
		wxCustomerMsg := &WxCustomerTextMsg{
			Touser:  msg.FromUserName,
			MsgType: msgType,
			Text:    *customerTextItem,
		}
		data, _ = json.Marshal(wxCustomerMsg)

	} else if msgType == "image" {
		customerImageItem := &WxCustomerImageItem{
			MediaId: content,
		}
		reqMsg := &WxCustomerImageMsg{
			Touser:  msg.FromUserName,
			MsgType: msgType,
			Image:   *customerImageItem,
		}
		data, _ = json.Marshal(reqMsg)
	} else {
		log.Warn("msgType not supported")
	}

	req, _ := http.NewRequest("POST", url, strings.NewReader(string(data)))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error(err)
		return
	}
	defer resp.Body.Close()

}

// 消息体拆分
func msgSpilt(content string) []string {
	var result []string

	n := len(content)/contentLimitSize + 1
	for i := 0; i < n; i++ {
		start := i * contentLimitSize
		end := (i + 1) * contentLimitSize
		if end > len(content) {
			end = len(content)
		}
		result = append(result, content[start:end])
	}
	return result
}

// 添加图片
func SendImage(msg *WxMsg, imageUrl string) {

	if imageUrl == "" {
		return
	}
	// 下载文件
	rsp, err := http.Get(imageUrl)
	if err != nil {
		log.Error(err)
		return
	}
	defer rsp.Body.Close()

	fileDir, _ := os.Getwd()
	fileName := time.Now().Unix()
	filePath := path.Join(fileDir, strconv.FormatInt(fileName, 10)+".jpg")

	// 输出文件
	file, _ := os.Create(filePath)
	io.Copy(file, rsp.Body)

	file, _ = os.Open(filePath)

	defer file.Close()
	defer os.Remove(filePath)

	// 素材上传
	mediaId := MediaUpload(filePath, file, "image")
	if mediaId != "" {
		// 发送客服消息
		SendCustomerMsgCore(mediaId, msg, "image")
	}

}

// 素材上传
func MediaUpload(filePath string, file *os.File, mediaType string) string {

	token := DefaultWechat().token
	url := fmt.Sprintf(mediaUploadUrl, token, mediaType)

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	part, err := writer.CreateFormFile("media", filepath.Base(filePath))
	if err != nil {
		log.Error(err)
		return ""
	}
	io.Copy(part, file)
	writer.Close()

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		log.Error(err)
		return ""
	}

	req.Header.Add("Content-Type", "multipart/form-data")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return ""
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Error(err)
		return ""
	}

	var rspMsg WxMediaRsp
	rspErr := json.Unmarshal(body, &rspMsg)
	if rspErr != nil {
		log.Error(rspErr)
		return ""
	}
	return rspMsg.MediaID
}

func MjNotifyHandler(c *gin.Context) {
	mjNotify := mj.NotifyHandler(c)

	if mjNotify != nil {
		wxMsg := &WxMsg{}

		err := json.Unmarshal([]byte(mjNotify.State), wxMsg)
		if err != nil {
			log.Error("序列化错误 err=%v\n", err)
			return
		}
		// 下载图片，发送客服消息
		SendImage(wxMsg, mjNotify.ImageUrl)
	}
}
