package main

import (
	"crypto/tls"
	"net/url"

	"gate.io/channel"
	"gate.io/job"
	"github.com/gateio/gateapi-go/v6"
	"github.com/gorilla/websocket"
)

var client *gateapi.APIClient
var socket *websocket.Conn

func init() {
	cfg := gateapi.NewConfiguration()
	cfg.Key = channel.Key
	cfg.Secret = channel.Secret
	client = gateapi.NewAPIClient(cfg)
	// client.ChangeBasePath("https://fx-api-testnet.gateio.ws/api/v4")

	u := url.URL{Scheme: "wss", Host: "api.gateio.ws", Path: "/ws/v4/"}
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{RootCAs: nil, InsecureSkipVerify: true}
	socket, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		panic(err)
	}
	socket.SetPingHandler(nil)
}

func main() {
	go job.NewSpotJob(channel.CurrencyPairBABY_USDT, 20, client, socket).Start()
	go job.NewSpotJob(channel.CurrencyPairAVT_USDT, 100, client, socket).Start()
	go job.NewSpotJob(channel.CurrencyPairCORE_USDT, 100, client, socket).Start()
	select {}
}
