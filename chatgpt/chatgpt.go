package chatgpt

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"wechat-mp-gpt/config"
	"wechat-mp-gpt/entity"
	"wechat-mp-gpt/user"

	log "github.com/sirupsen/logrus"
)

var (
	once       sync.Once
	defaultGPT *ChatGPT
)

type ChatGPT struct {
	config *config.Config
}

func DefaultGPT() *ChatGPT {
	// 单例模式，初始化
	once.Do(func() {
		defaultGPT = newChatGPT()
	})
	return defaultGPT
}

func newChatGPT() *ChatGPT {
	defer func() {
		if err := recover(); err != nil {
			log.Fatalln("初始化失败:", err)
		}
	}()
	gpt := &ChatGPT{
		config: config.ReadConfig(),
	}
	return gpt
}

// 返回 Result 类型的读通道
func (c *ChatGPT) SendMsgChan(openId string, msg string) <-chan entity.Result {
	// 创建通道  长度为1的缓冲
	ch := make(chan entity.Result, 1)
	// gorotuine + 匿名函数
	go func() {
		defer func() {
			if err := recover(); err != nil {
				err = err.(error)
			}
		}()
		ch <- entity.Result{
			Data: c.SendMsg(openId, msg),
		}
	}()
	return ch
}

func (c *ChatGPT) SendMsg(openId string, msg string) string {

	userInfo := user.UserInfo(openId)
	chatMsg := userInfo.GetMessage(msg)

	reqMsg := &entity.ChatReqMsg{
		Model:    c.config.OpenapiModel,
		Messages: chatMsg,
	}

	data, err := json.Marshal(reqMsg)
	if err != nil {
		log.Error("序列化错误 err=%v\n", err)
		return ""
	}

	req, _ := http.NewRequest("POST", c.config.OpenapiUrl, strings.NewReader(string(data)))

	req.Header.Set("Authorization", "Bearer "+c.config.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error(err)
		return ""
	}
	defer resp.Body.Close()

	respByte, _ := io.ReadAll(resp.Body)

	var rspMsg entity.ChatRspMsg

	rspErr := json.Unmarshal(respByte, &rspMsg)
	if rspErr != nil {
		log.Error(string(respByte))
		return ""
	}
	content := rspMsg.Choices[0].Message.Content
	userInfo.AppendMessage(content)
	return content
}

// 图片生成
func (c *ChatGPT) GenerateImage(prompt string) string {

	reqData := &entity.GptImageReq{
		Prompt: prompt,
		N:      1,
		Size:   "1024x1024",
	}

	data, err := json.Marshal(reqData)
	if err != nil {
		log.Error("gpt序列化错误 err=%v\n", err)
		return ""
	}

	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/images/generations", strings.NewReader(string(data)))

	req.Header.Set("Authorization", "Bearer "+c.config.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error(err)
		return ""
	}
	defer resp.Body.Close()

	respByte, _ := io.ReadAll(resp.Body)

	var rspMsg entity.GptImageRsp

	rspErr := json.Unmarshal(respByte, &rspMsg)
	if rspErr != nil {
		log.Error(string(respByte))
		return ""
	}
	log.Info("resp:%v\n", string(respByte))

	return rspMsg.Data[0].Url
}
