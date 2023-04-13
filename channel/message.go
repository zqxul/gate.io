package channel

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Message struct {
	Time    int64    `json:"time"`
	Channel string   `json:"channel"`
	Event   string   `json:"event"`
	Payload []string `json:"payload"`
	Auth    *Auth    `json:"auth"`
}

type Auth struct {
	Method string `json:"method"`
	KEY    string `json:"KEY"`
	SIGN   string `json:"SIGN"`
}

func sign(channel, event string, t int64, secret string) string {
	message := fmt.Sprintf("channel=%s&event=%s&time=%d", channel, event, t)
	h2 := hmac.New(sha512.New, []byte(secret))
	io.WriteString(h2, message)
	return hex.EncodeToString(h2.Sum(nil))
}

func (m *Message) Sign(key string, secret string) {
	signStr := sign(m.Channel, m.Event, m.Time, secret)
	m.Auth = &Auth{
		Method: "api_key",
		KEY:    key,
		SIGN:   signStr,
	}
}

func (m *Message) String() string {
	msgJson, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(msgJson)
}

func (m *Message) Send(ctx context.Context, c *websocket.Conn) error {
	return wsjson.Write(ctx, c, m)
}

func NewMsg(channel, event string, t int64, payload []string) *Message {
	return &Message{
		Time:    t,
		Channel: channel,
		Event:   event,
		Payload: payload,
	}
}
