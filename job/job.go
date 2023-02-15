package job

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
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
	OrderNum     int
}

func NewSpotJob(currencyPair string, client *gateapi.APIClient) *SpotJob {
	return &SpotJob{
		CurrencyPair: currencyPair,
		Client:       client,
		Gap:          decimal.NewFromFloat(0.01),
		OrderNum:     4,
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
	go job.refreshOrderBook(ctx)
	go job.fund(ctx, fund)
}

func (job SpotJob) refreshOrderBook(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			job.lookupMarketPrice(ctx)
			orders := job.currentOrders(ctx, channel.SpotChannelOrderSideBuy)
			log.Printf("*******************************************************[current orders]*******************************************************")
			for i, order := range orders {
				log.Printf("\t\t [%s]-[%s] with [price: %s, amount: %s/%s] was created at %s\n", order.Text, order.Side, order.Price, order.Left, order.Amount, time.UnixMilli(order.CreateTimeMs).Format("2006-01-02 15:04:05"))
				if i < len(orders)-1 {
					log.Printf("-----------------------------------------------------------------------------------------------------------------------------\n")
				}
			}
			log.Printf("*******************************************************[current orders]*******************************************************\n\n\n")
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
	log.Printf("job start with usdt account: [currency: %v, available: %s]", usdtAcct.Currency, usdtAcct.Available)
	_, bidPrice := job.lookupMarketPrice(ctx)
	amount := fund.Div(bidPrice).Div(decimal.NewFromInt(int64(job.OrderNum))).RoundFloor(6)
	if amount.LessThan(decimal.NewFromInt(1)) {
		log.Printf("order amount must be greater than 1, actual is %v. please add more fund or lower order num.", amount)
		ctx.Done()
		return
	}
	log.Printf("job start with - [price: %v, amount: %v]", bidPrice, amount)
	job.CreateBuyOrder(ctx, channel.SpotChannelOrderSideBuy, bidPrice, amount)
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

func (job *SpotJob) lookupMarketPrice(ctx context.Context) (decimal.Decimal, decimal.Decimal) {
	orderBook, _, err := job.Client.SpotApi.ListOrderBook(ctx, job.CurrencyPair, &gateapi.ListOrderBookOpts{
		Limit: optional.NewInt32(int32(job.OrderNum)),
	})
	if err != nil {
		panic(err)
	}
	askPrice, _ := decimal.NewFromString(orderBook.Asks[0][0])
	bidPrice, _ := decimal.NewFromString(orderBook.Bids[0][0])
	log.Printf("**************** %v - Ask :::::::::::::::::::: - Market - :::::::::::::::::::: Bid - %v ****************\n", orderBook.Asks[0], orderBook.Bids[0])
	return askPrice, bidPrice
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
		Limit: optional.NewInt32(int32(job.OrderNum)),
		Side:  optional.NewString(side),
	})
	if err != nil {
		log.Printf("handlePutEvent job.Client.SpotApi.ListOrders err: %v", err)
		return make([]gateapi.Order, 0)
	}
	return openOrders
}

func (job *SpotJob) CreateBuyOrder(ctx context.Context, side string, price, amount decimal.Decimal) {
	buyOrders := job.currentOrders(ctx, channel.SpotChannelOrderSideBuy)
	orderPrice := price.Mul(decimal.NewFromInt(1).Sub(job.Gap))

	// choose a better oder price
	if len(buyOrders) > 0 {
		sort.Slice(buyOrders, func(i, j int) bool {
			left, _ := decimal.NewFromString(buyOrders[i].Price)
			right, _ := decimal.NewFromString(buyOrders[j].Price)
			return left.GreaterThan(right)
		})
		topPrice, _ := decimal.NewFromString(buyOrders[0].Price)
		newTopPrice := topPrice.Mul(decimal.NewFromInt(1).Add(job.Gap))
		if newTopPrice.LessThan(orderPrice) {
			orderPrice = newTopPrice
		}
	}

	// create order
	orderAmount := price.Mul(decimal.NewFromInt(1).Sub(job.Gap)).Mul(amount)
	usdtAcct := job.account(ctx)
	accountAvailable, _ := decimal.NewFromString(usdtAcct.Available)
	log.Printf("start create buy order, [orderAmount: %v, accountAvailable: %v, current order num: %v, job order num: %v]", orderAmount, accountAvailable, len(buyOrders), job.OrderNum)
	if orderAmount.LessThan(accountAvailable) && len(buyOrders) < job.OrderNum && orderAmount.GreaterThanOrEqual(decimal.NewFromFloat(1)) {
		time.Sleep(1 * time.Second)
		_, _, err := job.Client.SpotApi.CreateOrder(ctx, gateapi.Order{
			Account:      "spot",
			Text:         fmt.Sprintf("t-%s", util.RandomID(10)),
			CurrencyPair: job.CurrencyPair,
			Side:         channel.SpotChannelOrderSideBuy,
			Price:        orderPrice.String(),
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
	job.CreateBuyOrder(ctx, channel.SpotChannelOrderSideBuy, order.Price, order.Amount)
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

func (job *SpotJob) OnOrderBuyed(ctx context.Context, order *channel.Order) {
	sellPrice := order.Price.Mul(decimal.NewFromInt(1).Add(job.Gap).Add(order.Fee.DivRound(order.Amount, 6)))
	newSellOrder, _, err := job.Client.SpotApi.CreateOrder(ctx, gateapi.Order{
		Account:      "spot",
		Text:         order.Id,
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

func (job *SpotJob) OnOrderSelled(ctx context.Context, order *channel.Order) {
	buyOrder, _, err := job.Client.SpotApi.GetOrder(ctx, order.Text, job.CurrencyPair, nil)
	if err != nil {
		log.Printf("OnOrderSelled get buy order err: %v", err)
		return
	}
	log.Printf("OnOrderSelled find buy order, [buy: %v, amount: %v]----[sell: %v, amount: %v]", buyOrder.Price, buyOrder.Account, order.Price, order.Amount)
	newOrderPrice, _ := decimal.NewFromString(buyOrder.Price)
	newOrderAmount, _ := decimal.NewFromString(buyOrder.Amount)
	job.CreateBuyOrder(ctx, channel.SpotChannelOrderSideBuy, newOrderPrice, newOrderAmount)
	if err != nil {
		log.Printf("OnOrderSelled job.Client.SpotApi.CreateOrder err: %v", err)
		return
	}
}
