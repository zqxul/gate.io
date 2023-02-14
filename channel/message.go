package channel

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gorilla/websocket"
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

func sign(channel, event string, t int64) string {
	message := fmt.Sprintf("channel=%s&event=%s&time=%d", channel, event, t)
	h2 := hmac.New(sha512.New, []byte(Secret))
	io.WriteString(h2, message)
	return hex.EncodeToString(h2.Sum(nil))
}

func (m *Message) Sign() {
	signStr := sign(m.Channel, m.Event, m.Time)
	m.Auth = &Auth{
		Method: "api_key",
		KEY:    Key,
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

func (m *Message) Send(c *websocket.Conn) error {
	msgByte, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return c.WriteMessage(websocket.TextMessage, msgByte)
}

func NewMsg(channel, event string, t int64, payload []string) *Message {
	return &Message{
		Time:    t,
		Channel: channel,
		Event:   event,
		Payload: payload,
	}
}
