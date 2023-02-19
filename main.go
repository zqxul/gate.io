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

var secondClient *gateapi.APIClient

func init() {
	cfg := gateapi.NewConfiguration()
	cfg.Key = channel.Key
	cfg.Secret = channel.Secret
	client = gateapi.NewAPIClient(cfg)
	// client.ChangeBasePath("https://fx-api-testnet.gateio.ws/api/v4")

	secondCfg := gateapi.NewConfiguration()
	secondCfg.Key = channel.SecondKey
	secondCfg.Secret = channel.SecondSecret
	secondClient = gateapi.NewAPIClient(secondCfg)
}

func GetSocket() *websocket.Conn {
	u := url.URL{Scheme: "wss", Host: "api.gateio.ws", Path: "/ws/v4/"}
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{RootCAs: nil, InsecureSkipVerify: true}
	socket, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		panic(err)
	}
	socket.SetPingHandler(nil)
	return socket
}

func main() {
	go job.NewSpotJob(channel.CurrencyPairDOGE_USDT, 45, client, GetSocket(), channel.Key).Start()
	go job.NewSpotJob(channel.CurrencyPairBIFI_USDT, 45, client, GetSocket(), channel.Key).Start()
	go job.NewSpotJob(channel.CurrencyPairBABY_USDT, 20, client, GetSocket(), channel.Key).Start()
	go job.NewSpotJob(channel.CurrencyPairAVT_USDT, 100, client, GetSocket(), channel.Key).Start()
	go job.NewSpotJob(channel.CurrencyPairBSW_USDT, 50, client, GetSocket(), channel.Key).Start()
	go job.NewSpotJob(channel.CurrencyPairCORE_USDT, 100, client, GetSocket(), channel.Key).Start()
	go job.NewSpotJob(channel.CurrencyPairMAPE_USDT, 15, secondClient, GetSocket(), channel.SecondKey).Start()
	select {}
}
