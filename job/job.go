package job

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gate.io/channel"
	"gate.io/util"
	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

type SpotJob struct {
	CurrencyPair string
	Client       *gateapi.APIClient
	Gap          decimal.Decimal
}

func NewSpotJob(currencyPair string, client *gateapi.APIClient) *SpotJob {
	return &SpotJob{
		CurrencyPair: currencyPair,
		Client:       client,
		Gap:          decimal.NewFromFloat(0.01),
	}
}

func (job *SpotJob) Start(fund decimal.Decimal, ws *websocket.Conn) {
	if err := job.subscribe(ws); err != nil {
		log.Printf("start job subscribe %s, err: %v", job.CurrencyPair, err)
		return
	}
	ctx := context.TODO()
	go job.beat(ctx, ws)
	go job.listen(ctx, ws)
	go job.fund(ctx, fund)
	go job.refreshOrders(ctx)
}

func (job SpotJob) refreshOrders(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			orders := job.currentOrders(ctx, channel.SpotChannelOrderSideBuy)
			for _, order := range orders {
				log.Printf("order[%s]-[%s] with [price: %s, amount: %s/%s] was created at %s\n", order.Text, order.Side, order.Price, order.Left, order.Amount, time.UnixMilli(order.CreateTimeMs))
			}
		case <-ctx.Done():
			return
		}
	}

}

func (job *SpotJob) account(ctx context.Context) gateapi.SpotAccount {
	accounts, _, err := job.Client.SpotApi.ListSpotAccounts(ctx, &gateapi.ListSpotAccountsOpts{
		Currency: optional.NewString("USDT"),
	})
	if err != nil {
		log.Printf("job start list spot account err: %v", err)
		return gateapi.SpotAccount{Currency: job.CurrencyPair}
	}
	if len(accounts) == 0 {
		panic("There has no spot account")
	}
	return accounts[0]
}

func (job *SpotJob) fund(ctx context.Context, fund decimal.Decimal) {
	usdtAcct := job.account(ctx)
	log.Printf("job start with usdt fund account: [currency: %v, available: %s]", usdtAcct.Currency, usdtAcct.Available)
	price, amount := job.getPriceAmount(ctx, job.CurrencyPair, fund)
	log.Printf("job start with [price: %v, amount: %v]", price, amount)
	job.CreateOrder(ctx, channel.SpotChannelOrderSideBuy, price, amount)
}

func (job *SpotJob) listen(ctx context.Context, ws *websocket.Conn) {
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			ws.Close()
			panic(err)
		}
		gateMessage := channel.GateMessage{}
		if err := json.Unmarshal([]byte(message), &gateMessage); err != nil {
			panic(err)
		}
		job.HandleMessage(ctx, &gateMessage)
	}
}

func (job *SpotJob) subscribe(ws *websocket.Conn) error {
	t := time.Now().Unix()
	ordersMsg := channel.NewMsg("spot.orders", "subscribe", t, []string{job.CurrencyPair})
	ordersMsg.Sign()
	return ordersMsg.Send(ws)
}

func (job *SpotJob) beat(ctx context.Context, ws *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t := time.Now().Unix()
			pingMsg := channel.NewMsg(channel.SpotChannelPing, "", t, []string{})
			if err := pingMsg.Send(ws); err != nil {
				log.Printf("job beat with ping err %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (job *SpotJob) getPriceAmount(ctx context.Context, currentPair string, fund decimal.Decimal) (decimal.Decimal, decimal.Decimal) {
	orderBook, _, err := job.Client.SpotApi.ListOrderBook(ctx, currentPair, &gateapi.ListOrderBookOpts{
		Limit: optional.NewInt32(10),
	})
	log.Printf("job getPriceAmount ListOrderBook success: %+v", orderBook)
	if err != nil {
		panic(err)
	}
	price, _ := decimal.NewFromString(orderBook.Bids[0][0])
	return price, fund.Div(price).Div(decimal.NewFromInt(2)).RoundFloor(6)
}

func (job *SpotJob) HandleMessage(ctx context.Context, message *channel.GateMessage) {
	switch message.Event {
	case channel.SpotChannelEventSubscribe:
		job.handleSubscribeEvent(ctx, message.ParseResult())
	case channel.SpotChannelEventUnsubscribe:
		job.handleUnscribeEvent(ctx, message.ParseResult())
	case channel.SpotChannelEventUpdate:
		job.handleUpdateEvent(ctx, message.ParseResult())
	}
}

func (job *SpotJob) handleSubscribeEvent(ctx context.Context, result string) {

}

func (job *SpotJob) handleUnscribeEvent(ctx context.Context, result string) {

}

func (job *SpotJob) handleUpdateEvent(ctx context.Context, result string) {
	orderResult := make(channel.OrderResult, 0)
	err := json.Unmarshal([]byte(result), &orderResult)
	if err != nil {
		log.Printf("handleUpdateEvent err: %v", err)
		return
	}
	for _, order := range orderResult {
		if order.CurrencyPair != job.CurrencyPair {
			return
		}
		switch order.Event {
		case channel.SpotChannelOrdersEventPut:
			job.handleOrderPutEvent(ctx, &order)
		case channel.SpotChannelEventUpdate:
			job.handleOrderUpdateEvent(ctx, &order)
		case channel.SpotChannelOrdersEventFinish:
			job.handleOrderFinishEvent(ctx, &order)
		}
	}
}

func (job *SpotJob) handleOrderUpdateEvent(ctx context.Context, order *channel.Order) {
	log.Printf("order [%s]-[%s] was updated: [price: %s, amount: %s/%s, fee: %s/%s]", order.Text, order.Side, order.Price, order.Amount.Sub(order.Left), order.Amount, order.Fee, order.FeeCurrency)

}

func (job *SpotJob) currentOrders(ctx context.Context, side string) []gateapi.Order {
	openOrders, _, err := job.Client.SpotApi.ListOrders(ctx, job.CurrencyPair, channel.SpotChannelOrdersStatusOpen, &gateapi.ListOrdersOpts{
		Limit: optional.NewInt32(10),
		Side:  optional.NewString(side),
	})
	if err != nil {
		log.Printf("handlePutEvent job.Client.SpotApi.ListOrders err: %v", err)
		return make([]gateapi.Order, 0)
	}
	return openOrders
}

func (job *SpotJob) CreateOrder(ctx context.Context, side string, price, amount decimal.Decimal) {
	buyOrders := job.currentOrders(ctx, channel.SpotChannelOrderSideBuy)
	orderAmount := price.Mul(decimal.NewFromInt(1).Sub(job.Gap)).Mul(amount)
	usdtAcct := job.account(ctx)
	accountAvailable, _ := decimal.NewFromString(usdtAcct.Available)
	if orderAmount.LessThan(accountAvailable) && len(buyOrders) < 10 && orderAmount.GreaterThanOrEqual(decimal.NewFromFloat(1)) {
		time.Sleep(1 * time.Second)
		_, _, err := job.Client.SpotApi.CreateOrder(ctx, gateapi.Order{
			Account:      "spot",
			Text:         fmt.Sprintf("t-%s", util.RandomID(10)),
			CurrencyPair: job.CurrencyPair,
			Side:         channel.SpotChannelOrderSideBuy,
			Price:        price.Mul(decimal.NewFromInt(1).Sub(job.Gap)).String(),
			Amount:       amount.String(),
		})
		if err != nil {
			log.Printf("handlePutEvent job.Client.SpotApi.CreateOrder err: %+v", err)
			return
		}
	}
}

func (job *SpotJob) handleOrderPutEvent(ctx context.Context, order *channel.Order) {
	log.Printf("Order [%s] was put, [price: %s, amount: %s]", order.Text, order.Price, order.Amount)
	job.CreateOrder(ctx, channel.SpotChannelOrderSideBuy, order.Price, order.Amount)
}

func (job *SpotJob) handleOrderFinishEvent(ctx context.Context, order *channel.Order) {
	if order.Left.GreaterThan(decimal.Zero) {
		log.Printf("Order [%s] was cancelled, [price: %s, amount: %s/%s]", order.Text, order.Price, order.Left, order.Amount)
		return
	}
	log.Printf("Order [%s] was closed, [price: %s, amount: %s, fee: %s/%s]", order.Text, order.Price, order.Amount, order.Fee, order.FeeCurrency)
	switch order.Side {
	case channel.SpotChannelOrderSideBuy:
		job.OnOrderBuyed(ctx, order)
	case channel.SpotChannelOrderSideSell:
		job.OnOrderSelled(ctx, order)
	}
}

// todo
func (job *SpotJob) OnOrderBuyed(ctx context.Context, order *channel.Order) {
	sellPrice := order.Price.Mul(decimal.NewFromInt(1).Add(decimal.NewFromFloat(0.8).Add(order.Fee)))
	newSellOrder, _, err := job.Client.SpotApi.CreateOrder(ctx, gateapi.Order{
		Account:      "spot",
		Text:         order.Text,
		CurrencyPair: job.CurrencyPair,
		Side:         channel.SpotChannelOrderSideSell,
		Price:        sellPrice.String(),
		Amount:       order.Amount.String(),
	})
	if err != nil {
		log.Printf("OnOrderBuyed job.Client.SpotApi.CreateOrder err:%v", err)
		return
	}
	log.Printf("OnOrderBuyed buy order success: %+v", newSellOrder)
}

// todo
func (job *SpotJob) OnOrderSelled(ctx context.Context, order *channel.Order) {
	openOrders, _, err := job.Client.SpotApi.ListOrders(ctx, job.CurrencyPair, channel.SpotChannelOrdersStatusOpen, &gateapi.ListOrdersOpts{
		Limit: optional.NewInt32(10),
		Side:  optional.NewString(channel.SpotChannelOrderSideBuy),
	})
	if err != nil {
		log.Printf("OnOrderSelled job.Client.SpotApi.ListOrders err: %v", err)
		return
	}
	var buyOrder *gateapi.Order
	for _, openOrder := range openOrders {
		if order.Text == openOrder.Text {
			buyOrder = &openOrder
		}
	}
	if buyOrder == nil {
		log.Printf("OnOrderSelled cannot find any buy order: %s", order.Text)
		return
	}
	log.Printf("OnOrderSelled find buy order, [buy: %v, amount: %v]----[sell: %v, amount: %v]", buyOrder.Price, buyOrder.Account, order.Price, order.Amount)

	nextOrderPrice, _ := decimal.NewFromString(buyOrder.Price)
	topOrderPrice, _ := decimal.NewFromString(openOrders[0].Price)
	if nextOrderPrice.GreaterThan(topOrderPrice) {
		nextOrderPrice = topOrderPrice
	}
	newBuyOrder, _, err := job.Client.SpotApi.CreateOrder(ctx, gateapi.Order{
		Account:      "spot",
		Text:         order.Text,
		CurrencyPair: job.CurrencyPair,
		Side:         channel.SpotChannelOrderSideBuy,
		Price:        nextOrderPrice.Mul(decimal.NewFromInt(1).Add(decimal.NewFromFloat(0.8))).String(),
		Amount:       order.Amount.String(),
	})
	if err != nil {
		log.Printf("OnOrderSelled job.Client.SpotApi.CreateOrder err: %v", err)
		return
	}
	log.Printf("OnOrderSelled create new order success: %+v", newBuyOrder)
}
