package mj

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"wechat-mp-gpt/config"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

type Midjourney struct {
	config *config.Config
}

type ImagineReq struct {
	Base64     string `json:"base64"`
	NotifyHook string `json:"notifyHook"`
	Prompt     string `json:"prompt"`
	State      string `json:"state"`
}

type SubmitRsp struct {
	Code        int16       `json:"code"`
	Description string      `json:"description"`
	Properties  interface{} `json:"properties"`
	Result      string      `json:"result"`
}

type MjNotify struct {
	Action      string `json:"action"`
	Id          string `json:"id"`
	Prompt      string `json:"prompt"`
	PromptEn    string `json:"promptEn"`
	Description string `json:"description"`
	SubmitTime  int64  `json:"submitTime"`
	StartTime   int64  `json:"startTime"`
	FinishTime  int64  `json:"finishTime"`
	ImageUrl    string `json:"imageUrl"`
	Status      string `json:"status"`
	State       string `json:"state"`
	Progress    string `json:"progress"`
	FailReason  string `json:"failReason"`
	// Properties        interface{} `json:"properties"`
	ProgressMessageId string `json:"progressMessageId"`
}

var (
	once       sync.Once
	defaultGPT *Midjourney
)

func DefaultMJ() *Midjourney {
	// 单例模式，初始化
	once.Do(func() {
		defaultGPT = newMJ()
	})
	return defaultGPT
}

func newMJ() *Midjourney {
	defer func() {
		if err := recover(); err != nil {
			log.Fatalln("初始化失败:", err)
		}
	}()
	mj := &Midjourney{
		config: config.ReadConfig(),
	}
	return mj
}

// 图片任务提交
func (mj *Midjourney) Imagine(prompt string, state string) {

	if mj.config.MidjourneyProxy == "" || mj.config.MidjourneyNotifyUrl == "" {
		log.Error("mj config wrong, please check it")
		return
	}

	body := &ImagineReq{
		Prompt:     prompt,
		NotifyHook: mj.config.MidjourneyNotifyUrl,
		State:      state,
	}

	data, err := json.Marshal(body)
	if err != nil {
		log.Error("序列化错误 err=%v\n", err)
		return
	}

	req, _ := http.NewRequest("POST", mj.config.MidjourneyProxy+"/mj/submit/imagine", strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error(err)
		return
	}
	defer resp.Body.Close()

	respByte, _ := io.ReadAll(resp.Body)

	rspMsg := &SubmitRsp{}

	rspErr := json.Unmarshal(respByte, &rspMsg)
	if rspErr != nil {
		log.Error(string(respByte))
		return
	}
	log.Info("mj imagine:%v\n", *rspMsg)
}

// 任务回调
func NotifyHandler(c *gin.Context) *MjNotify {

	data, _ := c.GetRawData()

	str := string(data)
	str = strings.ReplaceAll(str, "(MISSING)", "")
	data = []byte(str)

	mjNotify := &MjNotify{}

	err := json.Unmarshal(data, mjNotify)
	if err != nil {
		log.Info("mj序列化错误 err=%v\n", err)
		return nil
	}

	if mjNotify.Status == "SUCCESS" {

		log.WithFields(log.Fields{
			"mj-imageUrl": mjNotify.ImageUrl,
			"mj-state":    mjNotify.State,
		}).Info("mj state success")

		return mjNotify
	}
	return nil
}
