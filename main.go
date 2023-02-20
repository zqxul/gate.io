package main

import (
	"context"

	"gate.io/channel"
	"gate.io/job"
	"github.com/gateio/gateapi-go/v6"
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

func main() {
	// go job.NewSpotJob(channel.CurrencyPairDOGE_USDT, 50, channel.Key, channel.Secret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairBIFI_USDT, 50, channel.Key, channel.Secret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairBABY_USDT, 50, channel.Key, channel.Secret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairAVT_USDT, 50, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairBSW_USDT, 50, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairVGX_USDT, 50, channel.Key, channel.Secret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairCORE_USDT, 50, channel.Key, channel.Secret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairMAPE_USDT, 15, channel.SecondKey, channel.SecondSecret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairVGX_USDT, 65, channel.SecondKey, channel.SecondSecret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairBABY_USDT, 50, channel.SecondKey, channel.SecondKey).Start(context.TODO())
	select {}
}
