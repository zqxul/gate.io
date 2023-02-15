package main

import (
	"crypto/tls"
	"net/url"

	"gate.io/channel"
	"gate.io/job"
	"github.com/gateio/gateapi-go/v6"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

var client *gateapi.APIClient

func init() {
	cfg := gateapi.NewConfiguration()
	cfg.Key = channel.Key
	cfg.Secret = channel.Secret
	client = gateapi.NewAPIClient(cfg)
	// client.ChangeBasePath("https://fx-api-testnet.gateio.ws/api/v4")
}

func main() {
	spotJob := job.NewSpotJob(channel.CurrencyPairMAPE_USDT, client)
	u := url.URL{Scheme: "wss", Host: "api.gateio.ws", Path: "/ws/v4/"}
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{RootCAs: nil, InsecureSkipVerify: true}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		panic(err)
	}
	c.SetPingHandler(nil)
	spotJob.Start(decimal.NewFromFloat(4.87), c)

	select {}
}
