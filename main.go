package main

import (
	api "gate.io/api"
	_ "gate.io/api/job"
	"gate.io/channel"
	"gate.io/job"
	"github.com/shopspring/decimal"
)

func main() {
	go job.New(channel.CurrencyPairXRP_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.015), channel.SecondKey, channel.SecondSecret).Start()
	go job.New(channel.CurrencyPairEOS_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.015), channel.SecondKey, channel.SecondSecret).Start()
	go job.New(channel.CurrencyPairLTC_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.015), channel.SecondKey, channel.SecondSecret).Start()
	go job.New(channel.CurrencyPairVGX_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.020), channel.SecondKey, channel.SecondSecret).Start()
	go job.New(channel.CurrencyPairDOGE_USDT, decimal.NewFromFloat(50), decimal.NewFromFloat(0.015), channel.SecondKey, channel.SecondSecret).Start()
	api.Run()
	select {}
}
