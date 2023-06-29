package user

import (
	"fmt"
	"time"
	"wechat-mp-gpt/config"
	"wechat-mp-gpt/entity"

	"github.com/patrickmn/go-cache"
)

type Queue struct {
	items []interface{}
	size  int
}

func NewQueue(size int) *Queue {
	return &Queue{
		items: make([]interface{}, 0, size),
		size:  size,
	}
}

func (q *Queue) Enqueue(item interface{}) {
	if len(q.items) >= q.size {
		// 队列已满，先出队最先进队的数据
		q.Dequeue()
	}

	q.items = append(q.items, item)
}

func (q *Queue) Dequeue() interface{} {
	if len(q.items) == 0 {
		return nil
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item
}

func (q *Queue) IsEmpty() bool {
	return len(q.items) == 0
}

func (q *Queue) IsFull() bool {
	return len(q.items) == q.size
}

type User struct {
	OpenId  string
	Message *Queue
}

var (
	chatTokenCache *cache.Cache = cache.New(1*time.Hour, 2*time.Hour)
)

func UserInfo(openId string) *User {

	cacheToken, _ := chatTokenCache.Get(openId)

	var queue Queue
	len := config.ReadConfig().ChatTokenLimit
	if cacheToken == nil {
		queue = *NewQueue(len)
	} else {
		queue = cacheToken.(Queue)
	}

	return &User{
		OpenId:  openId,
		Message: &queue,
	}
}

func (user *User) refreshMessage(queue Queue) {
	// TODO 失效时间 1天？
	chatTokenCache.Set(user.OpenId, queue, cache.DefaultExpiration)
}

func (user *User) GetMessage(msg string) []entity.ChatMsg {

	chatMsg := entity.NewChatMsg("user", msg)
	queue := user.Message

	queue.Enqueue(*chatMsg)
	// save cache
	user.refreshMessage(*queue)

	return Convert2ChatMsg(queue.items)
}

/**
*  缓存回答消息
**/
func (user *User) AppendMessage(msg string) {
	chatMsg := entity.NewChatMsg("assistant", msg)
	queue := user.Message
	queue.Enqueue(*chatMsg)
	user.refreshMessage(*queue)
}

func Convert2ChatMsg(data []interface{}) []entity.ChatMsg {

	result := make([]entity.ChatMsg, len(data))

	for i, v := range data {
		if r, ok := v.(entity.ChatMsg); ok {
			result[i] = r
		} else {
			// 处理类型断言失败的情况
			fmt.Printf("Element at index %d is not a string\n", i)
		}
	}
	return result
}
