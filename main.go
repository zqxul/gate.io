package main

import (
	api "gate.io/api"
	_ "gate.io/api/job"
	"gate.io/channel"
	"gate.io/job"
	"github.com/shopspring/decimal"
)

func main() {
	// go job.NewSpotJob(channel.CurrencyPairDOGE_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairBIFI_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairBABY_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairAVT_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairBSW_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairVGX_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())
	// go job.NewSpotJob(channel.CurrencyPairCORE_USDT, 50, 0.003, channel.Key, channel.Secret).Start(context.TODO())

	// go job.NewSpotJob(channel.CurrencyPairMAPE_USDT, decimal.NewFromFloat(65), decimal.NewFromFloat(0.002), channel.SecondKey, channel.SecondSecret).Start()
	// go job.New(channel.CurrencyPairVGX_USDT, decimal.NewFromFloat(65), decimal.NewFromFloat(0.003), channel.SecondKey, channel.SecondSecret).Start()
	// go job.New(channel.CurrencyPairBABY_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.003), channel.SecondKey, channel.SecondSecret).Start()
	// go job.New(channel.CurrencyPairBLUR_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.003), channel.SecondKey, channel.SecondSecret).Start()
	// go job.New(channel.CurrencyPairAVT_USDT, decimal.NewFromFloat(100), decimal.NewFromFloat(0.003), channel.SecondKey, channel.SecondSecret).Start()
	// go job.New(channel.CurrencyPairBSW_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.003), channel.SecondKey, channel.SecondSecret).Start()
	// go job.New(channel.CurrencyPairCORE_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.003), channel.SecondKey, channel.SecondSecret).Start()
	// go job.New(channel.CurrencyPairXRP_USDT, decimal.NewFromFloat(288), decimal.NewFromFloat(0.0015), channel.SecondKey, channel.SecondSecret).Start()
	go job.New(channel.CurrencyPairEOS_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.015), channel.SecondKey, channel.SecondSecret).Start()
	go job.New(channel.CurrencyPairMATIC_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.015), channel.SecondKey, channel.SecondSecret).Start()
	go job.New(channel.CurrencyPairVGX_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.015), channel.SecondKey, channel.SecondSecret).Start()
	go job.New(channel.CurrencyPairLUNA_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.015), channel.SecondKey, channel.SecondSecret).Start()
	api.Run()
	select {}
}
