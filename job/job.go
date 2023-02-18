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
	CurrencyPair   string
	Client         *gateapi.APIClient
	Socket         *websocket.Conn
	Gap            decimal.Decimal
	OrderNum       int
	Fund           decimal.Decimal
	MinBaseAmount  decimal.Decimal
	MinQuoteAmount decimal.Decimal
	Account        gateapi.SpotAccount
}

func NewSpotJob(currencyPair string, fund float64, client *gateapi.APIClient, socket *websocket.Conn) *SpotJob {
	return &SpotJob{
		CurrencyPair: currencyPair,
		Client:       client,
		Gap:          decimal.NewFromFloat(0.005),
		OrderNum:     10,
		Fund:         decimal.NewFromFloat(fund),
		Socket:       socket,
	}
}

func (job *SpotJob) init(ctx context.Context) {
	job.refreshAccount(ctx)
	log.Printf("Spot Account: [Currency: %v, Available: %v]", job.Account.Currency, job.Account.Available)
	job.MinBaseAmount, job.MinQuoteAmount = job.getCurrencyPairMinAmount(ctx)
}

func (job *SpotJob) Start() {
	ctx := context.TODO()
	job.init(ctx)
	if err := job.subscribe(job.Socket); err != nil {
		log.Printf("start job subscribe %s, err: %v", job.CurrencyPair, err)
		return
	}
	go job.beat(ctx, job.Socket)
	go job.listen(ctx, job.Socket)
	go job.refreshOrderBook(ctx)
}

func (job SpotJob) refreshOrderBook(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			askPrice, _, bidPrice, _ := job.lookupMarketPrice(ctx)
			orders := job.currentOrders(ctx, "")
			if len(orders) == 0 {
				amount := job.Fund.Div(askPrice).Div(decimal.NewFromInt(int64(job.OrderNum))).RoundFloor(6)
				job.CreateBuyOrder(ctx, channel.SpotChannelOrderSideBuy, askPrice, amount)
			}
			log.Printf("**************** %v - Ask :::::::::::::::::::: - Market - :::::::::::::::::::: Bid - %v ****************\n", askPrice, bidPrice)
			log.Printf("*******************************************************[ %s ]*******************************************************", job.CurrencyPair)
			for i, order := range orders {
				log.Printf("\t\t [%s]-[%s] with [price: %s, amount: %s/%s] was created at %s\n", order.Text, order.Side, order.Price, order.Left, order.Amount, time.UnixMilli(order.CreateTimeMs).Format("2006-01-02 15:04:05"))
				if i < len(orders)-1 {
					log.Printf("-----------------------------------------------------------------------------------------------------------------------------\n")
				}
			}
			log.Printf("*******************************************************[ %s ]*******************************************************\n\n\n", job.CurrencyPair)
		case <-ctx.Done():
			return
		}
	}

}

func (job *SpotJob) refreshAccount(ctx context.Context) {
	accounts, _, err := job.Client.SpotApi.ListSpotAccounts(ctx, &gateapi.ListSpotAccountsOpts{
		Currency: optional.NewString("USDT"),
	})
	if err != nil {
		log.Printf("job start list spot account err: %v", err)
		return
	}
	if len(accounts) == 0 {
		panic("There has no spot account")
	}
	job.Account = accounts[0]
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

func (job *SpotJob) lookupMarketPrice(ctx context.Context) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {
	orderBook, _, err := job.Client.SpotApi.ListOrderBook(ctx, job.CurrencyPair, &gateapi.ListOrderBookOpts{
		Limit: optional.NewInt32(int32(job.OrderNum)),
	})
	if err != nil {
		panic(err)
	}
	askPrice, _ := decimal.NewFromString(orderBook.Asks[0][0])
	askAmount, _ := decimal.NewFromString(orderBook.Asks[0][1])
	bidPrice, _ := decimal.NewFromString(orderBook.Bids[0][0])
	bidAmount, _ := decimal.NewFromString(orderBook.Bids[0][1])
	return askPrice, askAmount, bidPrice, bidAmount
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

func (job *SpotJob) getCurrencyPairMinAmount(ctx context.Context) (decimal.Decimal, decimal.Decimal) {
	currencyPair, _, err := job.Client.SpotApi.GetCurrencyPair(ctx, job.CurrencyPair)
	if err != nil {
		log.Printf("get currency pair err: %v", err)
		ctx.Done()
		return decimal.Zero, decimal.Zero
	}
	log.Printf("Currency Pair: %+v", currencyPair)
	minBaseAmount, _ := decimal.NewFromString(currencyPair.MinBaseAmount)
	minQuoteAmount, _ := decimal.NewFromString(currencyPair.MinQuoteAmount)
	return minBaseAmount, minQuoteAmount
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

	job.refreshAccount(ctx)
	// create order
	orderAmount := price.Mul(decimal.NewFromInt(1).Sub(job.Gap)).Mul(amount)
	if orderAmount.LessThan(job.MinQuoteAmount) || amount.LessThan(job.MinBaseAmount) {
		return
	}
	accountAvailable, _ := decimal.NewFromString(job.Account.Available)
	log.Printf("Start create buy order, [orderAmount: %v, accountAvailable: %v, current buy order num: %v, job order num: %v]", orderAmount, accountAvailable, len(buyOrders), job.OrderNum)
	if orderAmount.LessThan(accountAvailable) && len(buyOrders) < job.OrderNum {
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
	job.refreshAccount(ctx)
	newSellOrder, _, err := job.Client.SpotApi.CreateOrder(ctx, gateapi.Order{
		Account:      "spot",
		Text:         fmt.Sprintf("t-%s", order.Id),
		CurrencyPair: job.CurrencyPair,
		Side:         channel.SpotChannelOrderSideSell,
		Price:        sellPrice.String(),
		Amount:       order.Amount.Sub(order.Fee).String(),
	})
	if err != nil {
		log.Printf("OnOrderBuyed job.Client.SpotApi.CreateOrder err:%v", err)
		return
	}
	log.Printf("OnOrderBuyed buy order success: %+v", newSellOrder)
}

func (job *SpotJob) OnOrderSelled(ctx context.Context, order *channel.Order) {
	buyOrder, _, err := job.Client.SpotApi.GetOrder(ctx, order.Text[2:], job.CurrencyPair, nil)
	if err != nil {
		log.Printf("OnOrderSelled get buy order err: %v", err)
		return
	}
	buyOrderPrice, _ := decimal.NewFromString(buyOrder.Price)
	buyOrderAmount, _ := decimal.NewFromString(buyOrder.Amount)
	buyOrderFee, _ := decimal.NewFromString(buyOrder.Fee)
	totalFee := buyOrderFee.Mul(buyOrderPrice).Add(order.Fee)
	profit := order.Price.Mul(order.Amount).Sub(buyOrderPrice.Mul(buyOrderAmount)).Sub(order.Fee)
	log.Printf("Deal %v [buy: %v, amount: %v]----[sell: %v, amount: %v]----[fee: %v, profit: %v]", order.CurrencyPair, buyOrder.Price, buyOrder.Amount, order.Price, order.Amount, totalFee, profit)
	newOrderPrice, _ := decimal.NewFromString(buyOrder.Price)
	newOrderAmount, _ := decimal.NewFromString(buyOrder.Amount)

	orders := job.currentOrders(ctx, channel.SpotChannelOrderSideBuy)
	if len(orders) > 0 && len(orders) < job.OrderNum {
		job.CreateBuyOrder(ctx, channel.SpotChannelOrderSideBuy, newOrderPrice, newOrderAmount)
		if err != nil {
			log.Printf("OnOrderSelled job.Client.SpotApi.CreateOrder err: %v", err)
			return
		}
	}
}
