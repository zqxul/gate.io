package channel

const (
	Key    = "ee9d2ec0f99f839394f00b302dc31df1"
	Secret = "7e03453df4eab061614f8cad5efd7f1a5580bf61b4b697d7a6287558f4d01d35"
)

const (
	SpotChannelPing    = "spot.ping"
	SpotChannelPong    = "spot.pong"
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
	CurrencyPairVGX_USDT  = "VGX_USDT"
	CurrencyPairXRP_USDT  = "XRP_USDT"
	CurrencyPairDOGE_USDT = "DOGE_USDT"
	CurrencyPairEOS_USDT  = "EOS_USDT"
	CurrencyPairLTC_USDT  = "LTC_USDT"
	CurrencyPairETH_USDT  = "ETH_USDT"
)

const (
	SpotChannelOrdersStatusOpen = "open"
)
