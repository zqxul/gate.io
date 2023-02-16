package channel

const (
	Key    = "6f809534c02fc7931c14135547270d01"
	Secret = "daa6a7f8c28fced4d46f6ac58b6d5545f10a35f863d6aeb4e1970281698b5614"
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
	CurrencyPairAVT_USDT  = "AVT_USDT"
	CurrencyPairBABY_USDT = "BABY_USDT"
	CurrencyPairXRP_USDT  = "XRP_USDT"
	CurrencyPairMAPE_USDT = "MAPE_USDT"
)

const (
	SpotChannelOrdersStatusOpen = "open"
)
