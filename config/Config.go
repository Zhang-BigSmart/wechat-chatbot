package config

import (
	"encoding/json"
	"os"
)

var (
	config = &Config{}
	// 在每次更新 config.json 时，需要执行的事件
	//configChangeCallbacks = make([]func(), 0)
)

type Config struct {
	OpenapiUrl          string   `json:"openapi-url"`
	OpenapiModel        string   `json:"openapi-model"`
	WhiteList           []string `json:"white-list"`
	ApiKey              string   `json:"api-key"`
	AllowAll            bool     `json:"allow-all"`
	SyncCustomer        bool     `json:"sync-customer"`
	WxAppId             string   `json:"wx-app-id"`
	WxAppSecret         string   `json:"wx-app-secret"`
	WxAppToken          string   `json:"wx-app-token"`
	ChatTokenLimit      int      `json:"chat-token-limit"`
	MidjourneyProxy     string   `json:"midjourney-proxy"`
	MidjourneyNotifyUrl string   `json:"midjourney-notify-url"`
}

func init() {
	readConfig()
}

func readConfig() {
	configFile, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}
	jsonParser := json.NewDecoder(configFile)

	if err = jsonParser.Decode(config); err != nil {
		panic(err)
	}
}

func ReadConfig() *Config {
	return config
}

func IsWhiteList(s string) bool {
	if config.AllowAll {
		return true
	}

	for _, item := range config.WhiteList {
		if item == s {
			return true
		}
	}
	return false
}
