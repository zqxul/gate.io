package main

import (
	"context"

	"gate.io/channel"
	"gate.io/job"
)

func main() {
	// go job.NewSpotJob(channel.CurrencyPairDOGE_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairBIFI_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairBABY_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairAVT_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairBSW_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairVGX_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairCORE_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())

	// go job.NewSpotJob(channel.CurrencyPairMAPE_USDT, 25, 0.003, channel.SecondKey, channel.SecondSecret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairVGX_USDT, 65, 0.003, channel.SecondKey, channel.SecondSecret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairBABY_USDT, 50, 0.003, channel.SecondKey, channel.SecondSecret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairBLUR_USDT, 50, 0.003, channel.SecondKey, channel.SecondSecret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairAVT_USDT, 50, 0.003, channel.SecondKey, channel.SecondSecret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairBSW_USDT, 50, 0.003, channel.SecondKey, channel.SecondSecret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairCORE_USDT, 50, 0.003, channel.SecondKey, channel.SecondSecret).Start(context.TODO())
	go job.NewSpotJob(channel.CurrencyPairXRP_USDT, 20, 0.0015, channel.SecondKey, channel.SecondSecret).Start(context.TODO())
	select {}
}
