package channel

const (
	Key    = "6f809534c02fc7931c14135547270d01"
	Secret = "daa6a7f8c28fced4d46f6ac58b6d5545f10a35f863d6aeb4e1970281698b5614"

	SecondKey    = "b5b582e3a6f62ccc4775e112cc85cc6a"
	SecondSecret = "06a847603eba515c39423b31b00627737244b170761886ea0c8530de1e153063"
)

const (
	SpotChannelPing    = "spot.ping"
	SpotChannelTickers = "spot.tickers"
	SpotChannelOrders  = "spot.orders" // auth required
)

const (
	SpotChannelEventSubscribe   = "subscribe"
	SpotChannelEventUpdate      = "update"
	SpotChannelEventUnsubscribe = "unsubscribe"
)

const (
	SpotChannelOrdersEventPut    = "put"
	SpotChannelOrdersEventUpdate = "update"
	SpotChannelOrdersEventFinish = "finish"
)

const (
	SpotChannelOrderSideBuy  = "buy"
	SpotChannelOrderSideSell = "sell"
)

const (
	CurrencyPairCORE_USDT = "CORE_USDT"
	CurrencyPairAVT_USDT  = "AVT_USDT"
	CurrencyPairBSW_USDT  = "BSW_USDT"
	CurrencyPairVGX_USDT  = "VGX_USDT"
	CurrencyPairBABY_USDT = "BABY_USDT"
	CurrencyPairBIFI_USDT = "BIFI_USDT"
	CurrencyPairXRP_USDT  = "XRP_USDT"
	CurrencyPairMAPE_USDT = "MAPE_USDT"
	CurrencyPairDOGE_USDT = "DOGE_USDT"
)

const (
	SpotChannelOrdersStatusOpen = "open"
)
