package wechat

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"wechat-mp-gpt/config"
	"wechat-mp-gpt/util/signature"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

var (
	once                sync.Once
	wechat              *Wechat
	customerUrlTemplate string = "https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token="
	tokenUrlTemplate    string = "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s"
	mediaUploadUrl      string = "https://api.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=%s"
)

type Wechat struct {
	token  string
	config *config.Config
}

type WxCheckReq struct {
	Signature string `form:"signature"`
	Timestamp string `form:"timestamp"`
	Nonce     string `form:"nonce"`
	Echostr   string `form:"echostr"`
}

type WxTokenRsp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func DefaultWechat() *Wechat {
	once.Do(func() {
		wechat = newWechat()
	})
	return wechat
}

func newWechat() *Wechat {
	defer func() {
		if err := recover(); err != nil {
			log.Fatalln("初始化失败:", err)
		}
	}()
	wechat := &Wechat{
		config: config.ReadConfig(),
	}
	return wechat
}

func Start() {

	DefaultWechat().tokenTask()

	r := gin.Default()
	r.GET("/wechat-check", check)
	r.POST("/wechat-check", wechatMsgReceive)
	r.POST("/mj/notify", MjNotifyHandler)
	r.Run()
}

// task:token refresh
func (wechat *Wechat) tokenTask() {
	if wechat.config.SyncCustomer {
		appId := wechat.config.WxAppId
		appSecret := wechat.config.WxAppSecret
		if len(appId) == 0 || len(appSecret) == 0 {
			panic(errors.New("wxAppId and WxAppSecret not null"))
		}
	}
	cronSpl := "0 0 * * * *"
	c := cron.New(cron.WithSeconds())
	go c.AddFunc(cronSpl, func() {
		wechat.tokenRefresh()
	})
	c.Start()
	wechat.tokenRefresh()
}

// token refresh
func (wechat *Wechat) tokenRefresh() {

	url := fmt.Sprintf(tokenUrlTemplate, wechat.config.WxAppId, wechat.config.WxAppSecret)
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("token refresh error")
		return
	}
	defer resp.Body.Close()

	respByte, _ := io.ReadAll(resp.Body)

	var tokenRsp WxTokenRsp

	rspErr := json.Unmarshal(respByte, &tokenRsp)
	if rspErr != nil {
		log.Error(string(respByte))
		return
	}

	wechat.token = tokenRsp.AccessToken
}

// wechat-check
func check(c *gin.Context) {
	var req WxCheckReq
	if c.ShouldBind(&req) == nil {
		log.WithFields(log.Fields{
			"Signature": req.Signature,
			"Timestamp": req.Timestamp,
			"Nonce":     req.Nonce,
			"Echostr":   req.Echostr,
		}).Info("wechat check parameters")
	}

	if signature.CheckSignature(req.Signature, req.Timestamp, req.Nonce, config.ReadConfig().WxAppToken) {
		c.String(200, req.Echostr)
	}

	log.Error("wechat signature check fail!")
}

// message receiver
func wechatMsgReceive(c *gin.Context) {
	b, _ := c.GetRawData()

	msg := &WxMsg{}

	err := xml.Unmarshal(b, &msg)
	if err != nil {
		log.Error("wechat raw msg unmarshal error")
		c.Writer.Write([]byte("success"))
		return
	}

	log.Info("receive msg:%+v\n", *msg)

	ProcessEvent(msg, c)
}
