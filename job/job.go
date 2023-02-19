package job

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"sort"
	"sync"
	"time"

	"gate.io/channel"
	"gate.io/util"
	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

var globalMux *sync.Mutex

type SpotJob struct {
	Client       *gateapi.APIClient
	Socket       *websocket.Conn
	CurrencyPair gateapi.CurrencyPair
	Gap          decimal.Decimal
	OrderAmount  decimal.Decimal
	OrderNum     int
	Fund         decimal.Decimal
	Key          string
	Secret       string
	mux          sync.Mutex
}

func getApiClient(ctx context.Context, key, secret string) *gateapi.APIClient {
	cfg := gateapi.NewConfiguration()
	cfg.Key = key
	cfg.Secret = secret
	return gateapi.NewAPIClient(cfg)
}

func getSocket(ctx context.Context) *websocket.Conn {
	u := url.URL{Scheme: "wss", Host: "api.gateio.ws", Path: "/ws/v4/"}
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{RootCAs: nil, InsecureSkipVerify: true}
	socket, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		panic(err)
	}
	socket.SetPingHandler(nil)
	return socket
}

func NewSpotJob(currencyPairId string, fund float64, key string, secret string) *SpotJob {
	ctx := context.TODO()
	job := &SpotJob{
		Client:       getApiClient(ctx, key, secret),
		Socket:       getSocket(ctx),
		Gap:          decimal.NewFromFloat(0.002),
		OrderNum:     10,
		OrderAmount:  decimal.NewFromFloat(fund).Div(decimal.NewFromInt(10)).RoundFloor(0),
		Key:          key,
		Secret:       secret,
		CurrencyPair: gateapi.CurrencyPair{Id: currencyPairId},
	}
	return job
}

func (job *SpotJob) init(ctx context.Context) {
	currencyPair, _, err := job.Client.SpotApi.GetCurrencyPair(ctx, job.CurrencyPair.Id)
	if err != nil {
		panic(err)
	}
	log.Printf("Currency Pair: %+v\n", currencyPair)
	job.CurrencyPair = currencyPair
}

func (job *SpotJob) Start(ctx context.Context) {
	time.Sleep(time.Second * time.Duration(rand.Intn(120)))
	job.init(ctx)
	log.Printf("[ %s ] job prepared", job.CurrencyPair.Base)
	job.subscribe()
	go job.beat(ctx, job.Socket)
	go job.listen(ctx, job.Socket)
	go job.refreshOrderBook(ctx)
	go job.refreshOrders(ctx)
}

func (job *SpotJob) getCurrencyAccount(ctx context.Context, currency string) *gateapi.SpotAccount {
	globalMux.Lock()
	defer globalMux.Unlock()

	accounts, _, err := job.Client.SpotApi.ListSpotAccounts(ctx, &gateapi.ListSpotAccountsOpts{
		Currency: optional.NewString(currency),
	})
	if err != nil {
		log.Printf("job start list spot account err: %v", err)
		return nil
	}
	if len(accounts) == 0 {
		return nil
	}
	account := accounts[0]
	log.Printf("Spot Account: [Currency: %v, Available: %v]\n", account.Currency, account.Available)
	return &account
}

func (job *SpotJob) refreshOrderBook(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			askPrice, _, bidPrice, _ := job.lookupMarketPrice(ctx)
			job.refreshOrders(ctx)
			orders := job.currentOrders(ctx, "")
			log.Printf("**************** %v - Ask :::::::::::::::::::: - Market - :::::::::::::::::::: Bid - %v ****************\n", askPrice, bidPrice)
			log.Printf("*******************************************************[ %s ]*******************************************************\n", job.CurrencyPair.Base)
			for i, order := range orders {
				log.Printf("\t\t [%s]-[%s] with [price: %s, amount: %s/%s] was created at %s\n", order.Text, order.Side, order.Price, order.Left, order.Amount, time.UnixMilli(order.CreateTimeMs).Format("2006-01-02 15:04:05"))
				if i < len(orders)-1 {
					log.Printf("-----------------------------------------------------------------------------------------------------------------------------\n")
				}
			}
			log.Printf("*******************************************************[ %s ]*******************************************************\n\n\n", job.CurrencyPair.Base)
		case <-ctx.Done():
			return
		}
	}
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

func (job *SpotJob) subscribe() {
	t := time.Now().Unix()
	ordersMsg := channel.NewMsg("spot.orders", "subscribe", t, []string{job.CurrencyPair.Id})
	ordersMsg.Sign(job.Key)
	if err := ordersMsg.Send(job.Socket); err != nil {
		panic(err)
	}
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
				log.Printf("job beat with ping err %v\n", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (job *SpotJob) lookupMarketPrice(ctx context.Context) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {
	orderBook, _, err := job.Client.SpotApi.ListOrderBook(ctx, job.CurrencyPair.Id, &gateapi.ListOrderBookOpts{
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
		log.Printf("handleUpdateEvent err: %v\n", err)
		return
	}
	for _, order := range orderResult {
		if order.CurrencyPair != job.CurrencyPair.Id {
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
	log.Printf("order [%s]-[%s]-[%s] was updated: [price: %s, amount: %s/%s, fee: %s/%s]\n", order.Text, order.Side, job.CurrencyPair.Base, order.Price, order.Amount.Sub(order.Left), order.Amount, order.Fee, order.FeeCurrency)
}

func (job *SpotJob) currentOrders(ctx context.Context, side string) []gateapi.Order {
	openOrders, _, err := job.Client.SpotApi.ListOrders(ctx, job.CurrencyPair.Id, channel.SpotChannelOrdersStatusOpen, &gateapi.ListOrdersOpts{
		Limit: optional.NewInt32(int32(job.OrderNum)),
		Side:  optional.NewString(side),
	})
	if err != nil {
		log.Printf("handlePutEvent job.Client.SpotApi.ListOrders err: %v\n", err)
		return make([]gateapi.Order, 0)
	}
	return openOrders
}

func (job *SpotJob) handleOrderPutEvent(ctx context.Context, order *channel.Order) {
	log.Printf("A new order [%s]-[%s]-[%s] was put, [price: %s, amount: %s]\n", order.Text, job.CurrencyPair.Base, order.Side, order.Price, order.Amount)
	job.refreshOrders(ctx)
}

func (job *SpotJob) refreshOrders(ctx context.Context) {
	job.mux.Lock()
	defer job.mux.Unlock()

	askPrice, _, bidPrice, _ := job.lookupMarketPrice(ctx)
	nextOrderPrice := decimal.Avg(askPrice, bidPrice).Mul(decimal.NewFromInt(1).Sub(job.Gap)).RoundFloor(job.CurrencyPair.Precision)

	// choose a better oder price
	buyOrders := job.currentOrders(ctx, channel.SpotChannelOrderSideBuy)
	if len(buyOrders) >= 10 {
		return
	}

	if len(buyOrders) > 0 {
		sort.Slice(buyOrders, func(i, j int) bool {
			left, _ := decimal.NewFromString(buyOrders[i].Price)
			right, _ := decimal.NewFromString(buyOrders[j].Price)
			return left.GreaterThan(right)
		})
		topOrderPrice, _ := decimal.NewFromString(buyOrders[0].Price)
		topOrderPrice = topOrderPrice.Mul(decimal.NewFromFloat(1).Add(job.Gap)).RoundFloor(job.CurrencyPair.Precision)
		if topOrderPrice.LessThan(nextOrderPrice) {
			bottomOrderPrice, _ := decimal.NewFromString(buyOrders[len(buyOrders)-1].Price)
			nextOrderPrice = bottomOrderPrice.Mul(decimal.NewFromInt(1).Sub(job.Gap)).RoundFloor(job.CurrencyPair.Precision)
		}
	}

	nextOrderAmount := job.OrderAmount.Div(nextOrderPrice).RoundFloor(job.CurrencyPair.AmountPrecision)
	minBaseAmount, _ := decimal.NewFromString(job.CurrencyPair.MinBaseAmount)
	minQuoteAmount, _ := decimal.NewFromString(job.CurrencyPair.MinQuoteAmount)
	if nextOrderAmount.LessThan(minBaseAmount) || job.OrderAmount.LessThan(minQuoteAmount) {
		return
	}

	quoteAcct := job.getCurrencyAccount(ctx, job.CurrencyPair.Quote)
	quoteAvailable, _ := decimal.NewFromString(quoteAcct.Available)
	nextOrderTotalAmount := nextOrderPrice.Mul(nextOrderAmount).Round(job.CurrencyPair.Precision)
	if quoteAvailable.LessThan(nextOrderTotalAmount) {
		return
	}

	if _, _, err := job.Client.SpotApi.CreateOrder(ctx, gateapi.Order{
		Account:      "spot",
		Text:         fmt.Sprintf("t-%s", util.RandomID(job.OrderNum)),
		CurrencyPair: job.CurrencyPair.Id,
		Side:         channel.SpotChannelOrderSideBuy,
		Price:        nextOrderPrice.String(),
		Amount:       nextOrderAmount.String(),
	}); err != nil {
		log.Printf("refreshOrders err: %+v\n", err)
		return
	}
}

func (job *SpotJob) handleOrderFinishEvent(ctx context.Context, order *channel.Order) {
	if order.Left.GreaterThan(decimal.Zero) {
		log.Printf("Order [%s] was cancelled, [price: %s, amount: %s/%s]\n", order.Text, order.Price, order.Left, order.Amount)
		return
	}
	log.Printf("Order [%s] was closed, [price: %s, amount: %s, fee: %s/%s]\n", order.Text, order.Price, order.Amount, order.Fee, order.FeeCurrency)
	switch order.Side {
	case channel.SpotChannelOrderSideBuy:
		job.OnOrderBuyed(ctx, order)
	case channel.SpotChannelOrderSideSell:
		job.OnOrderSelled(ctx, order)
	}
}

func (job *SpotJob) OnOrderBuyed(ctx context.Context, order *channel.Order) {
	log.Printf("Deal %v - [%s] [price: %v, amount: %v]----[fee: %v, left: %v]\n", job.CurrencyPair.Base, order.Side, order.Price, order.Amount, order.Fee.Mul(order.Price).RoundFloor(job.CurrencyPair.Precision), order.Amount.Sub(order.Fee).Round(job.CurrencyPair.AmountPrecision))
	sellPrice := order.Price.
		Mul(decimal.NewFromInt(1).
			Add(job.Gap).
			Add(order.Fee.Div(order.Amount.Sub(order.Fee)))).
		Round(job.CurrencyPair.Precision)
	_, _, err := job.Client.SpotApi.CreateOrder(ctx, gateapi.Order{
		Account:      "spot",
		Text:         fmt.Sprintf("t-%s", order.Id),
		CurrencyPair: job.CurrencyPair.Id,
		Side:         channel.SpotChannelOrderSideSell,
		Price:        sellPrice.String(),
		Amount:       order.Amount.Sub(order.Fee).String(),
	})
	if err != nil {
		log.Printf("OnOrderBuyed job.Client.SpotApi.CreateOrder err:%v\n", err)
		return
	}
}

func (job *SpotJob) OnOrderSelled(ctx context.Context, order *channel.Order) {
	buyOrder, _, err := job.Client.SpotApi.GetOrder(ctx, order.Text[2:], job.CurrencyPair.Id, nil)
	if err != nil {
		log.Printf("OnOrderSelled get buy order err: %v\n", err)
		return
	}
	buyOrderPrice, _ := decimal.NewFromString(buyOrder.Price)
	buyOrderAmount, _ := decimal.NewFromString(buyOrder.Amount)
	buyOrderFee, _ := decimal.NewFromString(buyOrder.Fee)
	totalFee := buyOrderFee.Mul(buyOrderPrice).Add(order.Fee)
	profit := order.Price.Mul(order.Amount).Sub(buyOrderPrice.Mul(buyOrderAmount)).Sub(order.Fee)
	log.Printf("Deal %v - [%s] [buy_price: %v, amount: %v]----[sell_price: %v, amount: %v]----[fee: %v, profit: %v]\n", job.CurrencyPair.Base, order.Side, buyOrder.Price, buyOrder.Amount, order.Price, order.Amount, totalFee, profit)
	job.refreshOrders(ctx)
}
