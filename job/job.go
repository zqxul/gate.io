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
	"strings"
	"sync"
	"time"

	"gate.io/channel"
	"gate.io/util"
	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

var globalMux sync.Mutex
var list []*SpotJob = make([]*SpotJob, 0)

type SpotJob struct {
	client       *gateapi.APIClient
	socket       *websocket.Conn
	CurrencyPair gateapi.CurrencyPair
	Gap          decimal.Decimal
	OrderAmount  decimal.Decimal
	OrderNum     int
	Fund         decimal.Decimal
	Key          string
	Secret       string
	mux          sync.Mutex
	State        [3]bool // [beat,socket,api]
	Stoped       bool
	ctx          context.Context
	trendDown    bool
}

func List() []*SpotJob {
	return list
}

func Edit(ID string, gap, orderAmount, fund decimal.Decimal, orderNum int) (exist bool) {
	item := Get(ID)
	if item == nil {
		return
	}
	item.Gap = gap
	item.OrderAmount = orderAmount
	item.OrderNum = orderNum
	item.Fund = fund
	return true
}

func Remove(ID string) (exist bool) {
	if item := Get(ID); item != nil {
		item.Stop()
		newJobs := make([]*SpotJob, 0)
		for _, item := range list {
			if item.CurrencyPair.Id != ID {
				newJobs = append(newJobs, item)
			}
		}
		list = newJobs
	}
	return
}

func Get(ID string) *SpotJob {
	for _, item := range list {
		if item.CurrencyPair.Id == ID {
			return item
		}
	}
	return nil
}

func Stop(ID string) (exist bool) {
	item := Get(ID)
	if item != nil {
		if !item.Stoped {
			item.Stop()
		}
		return true
	}
	return
}

func Resume(ID string) (exist bool) {
	item := Get(ID)
	if item != nil {
		if item.Stoped {
			item.Start()
		}
		return true
	}
	return
}

func getApiClient(key, secret string) *gateapi.APIClient {
	cfg := gateapi.NewConfiguration()
	cfg.Key = key
	cfg.Secret = secret
	return gateapi.NewAPIClient(cfg)
}

func getSocket() *websocket.Conn {
	u := url.URL{Scheme: "wss", Host: "api.gateio.ws", Path: "/ws/v4/"}
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{RootCAs: nil, InsecureSkipVerify: true}
	socket, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		panic(err)
	}
	socket.SetPingHandler(nil)
	return socket
}

func New(currencyPairId string, fund, gap decimal.Decimal, key string, secret string) *SpotJob {
	var job *SpotJob = Get(currencyPairId)
	if job != nil {
		return job
	}
	job = &SpotJob{
		Gap:          gap,
		OrderNum:     10,
		Fund:         fund,
		OrderAmount:  fund.Div(decimal.NewFromInt(10)).RoundFloor(0),
		Key:          key,
		Secret:       secret,
		CurrencyPair: gateapi.CurrencyPair{Id: currencyPairId},
		ctx:          context.TODO(),
		Stoped:       true,
	}
	list = append(list, job)
	return job
}

func (sj *SpotJob) init() {
	sj.client = getApiClient(sj.Key, sj.Secret)
	sj.socket = getSocket()
	currencyPair, _, err := sj.client.SpotApi.GetCurrencyPair(sj.ctx, sj.CurrencyPair.Id)
	if err != nil {
		panic(err)
	}
	log.Printf("Currency Pair: %+v\n", currencyPair)
	sj.CurrencyPair = currencyPair
	sj.State = [3]bool{}
	sj.Stoped = false
}

func (sj *SpotJob) restart() {
	sj.Stop()
	sj.Start()
}

func (sj *SpotJob) Start() {
	sj.init()
	sj.subscribe()
	log.Printf("[ %s ] job started", sj.CurrencyPair.Base)

	go sj.beat()
	go sj.listen()

	go sj.refresh()
}

func (sj *SpotJob) Stop() {
	sj.Stoped = true
	sj.State = [3]bool{}
	sj.ctx.Done()
	sj.unsubscribe()
	sj.socket.Close()
}

func (sj *SpotJob) getCurrencyAccount(currency string) gateapi.SpotAccount {
	globalMux.Lock()
	defer globalMux.Unlock()

	accounts, _, err := sj.client.SpotApi.ListSpotAccounts(sj.ctx, &gateapi.ListSpotAccountsOpts{
		Currency: optional.NewString(currency),
	})
	if err != nil {
		sj.State[2] = false
		log.Printf("job start list spot account err: %v", err)
		return gateapi.SpotAccount{}
	}
	sj.State[2] = true
	if len(accounts) == 0 {
		return gateapi.SpotAccount{}
	}
	account := accounts[0]
	return account
}

func (sj *SpotJob) refresh() {
	ticker := time.NewTicker(time.Duration(rand.Intn(90)) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			go sj.refreshOrders()
			go sj.refreshOrderBook()
			go sj.refreshMarket()
		case <-sj.ctx.Done():
			return
		}
	}
}

func (sj *SpotJob) refreshMarket() {
	time.Sleep(time.Duration(60+rand.Intn(10)) * time.Second)
	sj.mux.Lock()
	defer sj.mux.Unlock()

	now := time.Now()
	from, to := now.Add(-time.Hour).Unix(), now.Unix()
	result, _, err := sj.client.SpotApi.ListCandlesticks(sj.ctx, sj.CurrencyPair.Id, &gateapi.ListCandlesticksOpts{
		From:     optional.NewInt64(from),
		To:       optional.NewInt64(to),
		Interval: optional.NewString("15m"),
		Limit:    optional.NewInt32(15),
	})
	if err != nil {
		log.Printf("ListCandlesticks err: %v\n", err)
	}
	start, _ := decimal.NewFromString(result[0][2])
	end, _ := decimal.NewFromString(result[len(result)-1][2])

	buyOrders := sj.currentOrders(channel.SpotChannelOrderSideBuy)
	sort.Slice(buyOrders, func(i, j int) bool {
		leftPrice, _ := decimal.NewFromString(buyOrders[i].Price)
		rightPrice, _ := decimal.NewFromString(buyOrders[j].Price)
		return leftPrice.LessThanOrEqual(rightPrice)
	})

	sellOrders := sj.currentOrders(channel.SpotChannelOrderSideSell)

	if start.LessThan(end) && len(sellOrders) <= 1 {
		sj.trendDown = false
		_, _, err := sj.client.SpotApi.CancelOrder(sj.ctx, buyOrders[0].Id, sj.CurrencyPair.Id, &gateapi.CancelOrderOpts{})
		if err != nil {
			log.Printf("CancelOrder err: %v", err)
		}
	} else if start.GreaterThan(end) {
		sj.trendDown = true
	}
}

func (sj *SpotJob) refreshOrderBook() {
	account := sj.getCurrencyAccount(sj.CurrencyPair.Quote)
	askPrice, _, bidPrice, _, err := sj.lookupMarketPrice()
	if err != nil {
		sj.State[2] = false
		return
	}
	sj.State[2] = true
	orders := sj.currentOrders("")
	fmt.Printf("\n\n")
	log.Printf("%s\n", strings.Repeat("*", 185))
	log.Printf("%s [Currency: %-20s            Available: %-20s] %s\n", strings.Repeat("*", 54), account.Currency, account.Available, strings.Repeat("*", 54))
	log.Printf("%s %-10s - Ask :::::::::::::::::: - Market - :::::::::::::::::: Bid - %10s %s\n", strings.Repeat("*", 51), askPrice, bidPrice, strings.Repeat("*", 51))
	log.Printf("%s\n", strings.Repeat("*", 185))
	log.Printf("| %20s | %20s | %20s | %20s | %20s | %20s | %20s | %20s |\n", "CURRENCY", "ID", "TEXT", "SIDE", "PRICE", "AMOUNT", "LEFT", "TIME")
	log.Printf("%s\n", strings.Repeat("*", 185))
	for i, order := range orders {
		log.Printf("| %20s | %20s | %20s | %20s | %20s | %20s | %20s | %20s |\n", sj.CurrencyPair.Base, order.Id, order.Text, order.Side, order.Price, order.Amount, order.Left, time.UnixMilli(order.CreateTimeMs).Format("2006-01-02 15:04:05"))
		if i < len(orders)-1 {
			log.Printf("*%s*\n", strings.Repeat("-", 183))
		}
	}
	log.Printf("%s\n", strings.Repeat("*", 185))
	fmt.Printf("\n\n")
}

func (sj *SpotJob) listen() {
	for {
		if sj.Stoped {
			return
		}
		_, message, err := sj.socket.ReadMessage()
		if err != nil {
			sj.State[1] = false
			log.Printf("job [%s] read socket message err: %v\n", sj.CurrencyPair.Base, err)
			sj.restart()
			log.Printf("job [%s] restart", sj.CurrencyPair.Base)
			continue
		}
		gateMessage := channel.GateMessage{}
		if err := json.Unmarshal([]byte(message), &gateMessage); err != nil {
			sj.State[1] = false
			log.Printf("job [%s], unmarshal gate message: %s err: %v\n", sj.CurrencyPair.Base, message, err)
			continue
		}
		sj.State[1] = true
		sj.HandleMessage(&gateMessage)
	}
}

func (sj *SpotJob) subscribe() {
	t := time.Now().Unix()
	ordersMsg := channel.NewMsg("spot.orders", "subscribe", t, []string{sj.CurrencyPair.Id})
	ordersMsg.Sign(sj.Key, sj.Secret)
	if err := ordersMsg.Send(sj.socket); err != nil {
		panic(err)
	}
}

func (sj *SpotJob) unsubscribe() {
	t := time.Now().Unix()
	ordersMsg := channel.NewMsg("spot.orders", "unsubscribe", t, []string{sj.CurrencyPair.Id})
	ordersMsg.Sign(sj.Key, sj.Secret)
	if err := ordersMsg.Send(sj.socket); err != nil {
		log.Printf("unsubscribe err: %v\n", err)
	}
}

func (sj *SpotJob) beat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t := time.Now().Unix()
			pingMsg := channel.NewMsg(channel.SpotChannelPing, "", t, []string{})
			if err := pingMsg.Send(sj.socket); err != nil {
				sj.State[0] = false
				log.Printf("job [%s] ping err %v\n", sj.CurrencyPair.Base, err)
				continue
			}
			sj.State[0] = true
		case <-sj.ctx.Done():
			return
		}
	}
}

func (sj *SpotJob) lookupMarketPrice() (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {
	orderBook, _, err := sj.client.SpotApi.ListOrderBook(sj.ctx, sj.CurrencyPair.Id, &gateapi.ListOrderBookOpts{
		Limit: optional.NewInt32(int32(sj.OrderNum)),
	})
	if err != nil {
		sj.State[2] = false
		log.Printf("lookupMarketPrice err: %v\n", err)
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, err
	}
	sj.State[2] = true
	askPrice, _ := decimal.NewFromString(orderBook.Asks[0][0])
	askAmount, _ := decimal.NewFromString(orderBook.Asks[0][1])
	bidPrice, _ := decimal.NewFromString(orderBook.Bids[0][0])
	bidAmount, _ := decimal.NewFromString(orderBook.Bids[0][1])
	return askPrice, askAmount, bidPrice, bidAmount, nil
}

func (sj *SpotJob) HandleMessage(message *channel.GateMessage) {
	switch message.Event {
	case channel.SpotChannelEventSubscribe:
		sj.handleSubscribeEvent(message.ParseResult())
	case channel.SpotChannelEventUnsubscribe:
		sj.handleUnscribeEvent(message.ParseResult())
	case channel.SpotChannelEventUpdate:
		sj.handleUpdateEvent(message.ParseResult())
	}
}

func (job *SpotJob) handleSubscribeEvent(result string) {

}

func (job *SpotJob) handleUnscribeEvent(result string) {

}

func (sj *SpotJob) handleUpdateEvent(result string) {
	orderResult := make(channel.OrderResult, 0)
	err := json.Unmarshal([]byte(result), &orderResult)
	if err != nil {
		log.Printf("handleUpdateEvent err: %v\n", err)
		return
	}
	for _, order := range orderResult {
		if order.CurrencyPair != sj.CurrencyPair.Id {
			return
		}
		switch order.Event {
		case channel.SpotChannelOrdersEventPut:
			sj.handleOrderPutEvent(&order)
		case channel.SpotChannelEventUpdate:
			sj.handleOrderUpdateEvent(&order)
		case channel.SpotChannelOrdersEventFinish:
			sj.handleOrderFinishEvent(&order)
		}
	}
}

func (sj *SpotJob) handleOrderUpdateEvent(order *channel.Order) {
	log.Printf("order [%s]-[%s]-[%s] was updated: [price: %s, amount: %s/%s, fee: %s/%s]\n", order.Text, order.Side, sj.CurrencyPair.Base, order.Price, order.Amount.Sub(order.Left), order.Amount, order.Fee, order.FeeCurrency)
}

func (sj *SpotJob) currentOrders(side string) []gateapi.Order {
	openOrders, _, err := sj.client.SpotApi.ListOrders(sj.ctx, sj.CurrencyPair.Id, channel.SpotChannelOrdersStatusOpen, &gateapi.ListOrdersOpts{
		Limit: optional.NewInt32(int32(sj.OrderNum)),
		Side:  optional.NewString(side),
	})
	if err != nil {
		sj.State[2] = false
		log.Printf("handlePutEvent sj.client.SpotApi.ListOrders err: %v\n", err)
		return make([]gateapi.Order, 0)
	}
	sj.State[2] = true
	return openOrders
}

func (sj *SpotJob) handleOrderPutEvent(order *channel.Order) {
	log.Printf("A new order [%s]-[%s]-[%s] was put, [price: %s, amount: %s]\n", order.Text, sj.CurrencyPair.Base, order.Side, order.Price, order.Amount)
	sj.refreshOrders()
}

func (sj *SpotJob) refreshOrders() {
	time.Sleep(time.Duration(10+rand.Intn(10)) * time.Second)
	sj.mux.Lock()
	defer sj.mux.Unlock()

	askPrice, _, bidPrice, _, err := sj.lookupMarketPrice()
	if err != nil {
		sj.State[2] = false
		return
	}
	rate := sj.Gap
	if sj.trendDown {
		rate = rate.Mul(decimal.NewFromFloat(2))
	}
	nextRate := decimal.NewFromInt(1).Sub(rate).RoundUp(3)
	nextOrderPrice := decimal.Min(askPrice, bidPrice).Mul(nextRate).RoundFloor(sj.CurrencyPair.Precision)

	// choose a better oder price
	buyOrders := sj.currentOrders(channel.SpotChannelOrderSideBuy)
	if len(buyOrders) >= 10 {
		return
	}

	if len(buyOrders) > 0 {
		prices := make([]decimal.Decimal, 0)
		for _, order := range buyOrders {
			price, _ := decimal.NewFromString(order.Price)
			prices = append(prices, price)
		}
		topPrice := decimal.Max(prices[0], prices...)
		bottomPrice := decimal.Min(prices[0], prices...)
		if nextTopPrice := topPrice.Mul(decimal.NewFromFloat(1).Add(rate)).RoundFloor(sj.CurrencyPair.Precision); nextTopPrice.LessThanOrEqual(nextOrderPrice) && !sj.trendDown {
			nextOrderPrice = nextTopPrice
		} else {
			nextBottomPrice := bottomPrice.Mul(decimal.NewFromFloat(1).Sub(rate)).RoundFloor(sj.CurrencyPair.Precision)
			nextOrderPrice = nextBottomPrice
		}
	}

	nextOrderAmount := sj.OrderAmount.Div(nextOrderPrice).RoundFloor(sj.CurrencyPair.AmountPrecision)
	minBaseAmount, _ := decimal.NewFromString(sj.CurrencyPair.MinBaseAmount)
	minQuoteAmount, _ := decimal.NewFromString(sj.CurrencyPair.MinQuoteAmount)
	if nextOrderAmount.LessThan(minBaseAmount) || sj.OrderAmount.LessThan(minQuoteAmount) {
		return
	}

	quoteAcct := sj.getCurrencyAccount(sj.CurrencyPair.Quote)
	quoteAvailable, _ := decimal.NewFromString(quoteAcct.Available)
	nextOrderTotalAmount := nextOrderPrice.Mul(nextOrderAmount).Round(sj.CurrencyPair.Precision)
	if quoteAvailable.LessThan(nextOrderTotalAmount) {
		return
	}

	if _, _, err := sj.client.SpotApi.CreateOrder(sj.ctx, gateapi.Order{
		Account:      "spot",
		Text:         fmt.Sprintf("t-%s", util.RandomID(sj.OrderNum)),
		CurrencyPair: sj.CurrencyPair.Id,
		Side:         channel.SpotChannelOrderSideBuy,
		Price:        nextOrderPrice.String(),
		Amount:       nextOrderAmount.String(),
	}); err != nil {
		sj.State[2] = false
		log.Printf("[ %s ] refreshOrders err: %+v\n", sj.CurrencyPair.Base, err)
		return
	}
	sj.State[2] = true
}

func (sj *SpotJob) handleOrderFinishEvent(order *channel.Order) {
	if order.Left.GreaterThan(decimal.Zero) {
		log.Printf("Order [%s] was cancelled, [price: %s, amount: %s/%s]\n", order.Text, order.Price, order.Left, order.Amount)
		return
	}
	log.Printf("Order [%s] was closed, [price: %s, amount: %s, fee: %s/%s]\n", order.Text, order.Price, order.Amount, order.Fee, order.FeeCurrency)
	switch order.Side {
	case channel.SpotChannelOrderSideBuy:
		sj.OnOrderBuyed(order)
	case channel.SpotChannelOrderSideSell:
		sj.OnOrderSelled(order)
	}
}

func (sj *SpotJob) OnOrderBuyed(order *channel.Order) {
	log.Printf("Deal %v - [%s] [price: %v, amount: %v, left: %v]----[fee: %v, left: %v]\n", sj.CurrencyPair.Base, order.Side, order.Price, order.Amount, order.Left, order.Fee.Mul(order.Price).RoundFloor(sj.CurrencyPair.Precision), order.Amount.Sub(order.Fee).Round(sj.CurrencyPair.AmountPrecision))
	sellPrice := order.Price.
		Mul(decimal.NewFromInt(1).
			Add(sj.Gap).
			Add(order.Fee.Add(order.PointFee).Add(order.GtFee).Add(order.RebatedFee).RoundUp(3).Div(order.Amount).RoundUp(3).Mul(decimal.NewFromInt(2)))).
		Round(sj.CurrencyPair.Precision)
	_, _, err := sj.client.SpotApi.CreateOrder(sj.ctx, gateapi.Order{
		Account:      "spot",
		Text:         fmt.Sprintf("t-%s", order.Id),
		CurrencyPair: sj.CurrencyPair.Id,
		Side:         channel.SpotChannelOrderSideSell,
		Price:        sellPrice.String(),
		Amount:       order.Amount.Sub(order.Fee).String(),
	})
	if err != nil {
		log.Printf("OnOrderBuyed sj.client.SpotApi.CreateOrder err:%v\n", err)
		return
	}
}

func (sj *SpotJob) OnOrderSelled(order *channel.Order) {
	buyOrder, _, err := sj.client.SpotApi.GetOrder(sj.ctx, order.Text[2:], sj.CurrencyPair.Id, nil)
	if err != nil {
		log.Printf("OnOrderSelled get buy order err: %v\n", err)
		return
	}
	buyOrderPrice, _ := decimal.NewFromString(buyOrder.Price)
	buyOrderAmount, _ := decimal.NewFromString(buyOrder.Amount)
	buyOrderFee, _ := decimal.NewFromString(buyOrder.Fee)
	totalFee := buyOrderFee.Mul(buyOrderPrice).Add(order.Fee)
	profit := order.Price.Mul(order.Amount).Sub(buyOrderPrice.Mul(buyOrderAmount)).Sub(order.Fee)
	log.Printf("Deal %v - [%s] [buy_price: %v, amount: %v, fee: %v]----[sell_price: %v, amount: %v, fee: %v]----[fee: %v, profit: %v]\n", sj.CurrencyPair.Base, order.Side, buyOrder.Price, buyOrder.Amount, buyOrderPrice.Mul(buyOrderFee), order.Price, order.Amount, order.Fee, totalFee, profit)
	sj.refreshOrders()
}
