package channel

const (
	Key    = "6f809534c02fc7931c14135547270d01"
	Secret = "daa6a7f8c28fced4d46f6ac58b6d5545f10a35f863d6aeb4e1970281698b5614"

	SecondKey    = "a44e87e3eb6eac14f5250262257439c5"
	SecondSecret = "6633e8cb23bfe8fa3a4c42ee05d4355bf80d0663585fb7d2cfef1c6a845ac047"
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
	CurrencyPairCORE_USDT  = "CORE_USDT"
	CurrencyPairAVT_USDT   = "AVT_USDT"
	CurrencyPairBSW_USDT   = "BSW_USDT"
	CurrencyPairVGX_USDT   = "VGX_USDT"
	CurrencyPairBABY_USDT  = "BABY_USDT"
	CurrencyPairBLUR_USDT  = "BLUR_USDT"
	CurrencyPairBIFI_USDT  = "BIFI_USDT"
	CurrencyPairXRP_USDT   = "XRP_USDT"
	CurrencyPairMAPE_USDT  = "MAPE_USDT"
	CurrencyPairDOGE_USDT  = "DOGE_USDT"
	CurrencyPairARB_USDT   = "ARB_USDT"
	CurrencyPairTEM_USDT   = "TEM_USDT"
	CurrencyPairEOS_USDT   = "EOS_USDT"
	CurrencyPairMATIC_USDT = "MATIC_USDT"
	CurrencyPairLUNA_USDT  = "LUNA_USDT"
)

const (
	SpotChannelOrdersStatusOpen = "open"
)
