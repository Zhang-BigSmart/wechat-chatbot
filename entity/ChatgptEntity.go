package entity

type ChatRspMsg struct {
	Id string `json:"id"`
	// Object string `json:object`
	Created int           `json:"created"`
	Model   string        `json:"model"`
	Choices []ChatChoices `json:"choices"`
}

type ChatChoices struct {
	Message      ChatMsg `json:"message"`
	FinishReason string  `json:"finish_reason"`
	Index        int32   `json:"index"`
}

type Result struct {
	Data string
	Err  error
}

type ChatReqMsg struct {
	Model    string    `json:"model"`
	Messages []ChatMsg `json:"messages"`
}

type ChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewChatMsg(role string, msg string) *ChatMsg {
	return &ChatMsg{
		Role:    role,
		Content: msg,
	}
}

type GptImageReq struct {
	Prompt string `json:"prompt"`
	N      int16  `json:"n"`
	Size   string `json:"size"`
}

type GptImageRsp struct {
	Created int64             `json:"created"`
	Data    []GptImageRspItem `json:"data"`
}

type GptImageRspItem struct {
	Url string `json:"url"`
}
