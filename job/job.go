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
	socketMux    sync.Mutex
	Up, Down     int
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
	socket.SetPingHandler(func(appData string) error {
		Pong(socket)
		return nil
	})
	return socket
}

func Pong(socket *websocket.Conn) {
	t := time.Now().Unix()
	pongMsg := channel.NewMsg(channel.SpotChannelPong, "", t, []string{})
	if err := pongMsg.Pong(socket); err != nil {
		log.Printf("job ping err %v\n", err)
		return
	}
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
	ticker := time.NewTicker(30 + time.Duration(sj.getRandomSecond(60))*time.Second)
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

type Trend struct {
	Start300 decimal.Decimal
	End300   decimal.Decimal
	Start100 decimal.Decimal
	End100   decimal.Decimal
	Start25  decimal.Decimal
	End25    decimal.Decimal
}

func (trend Trend) Up() bool {
	return trend.Start300.LessThan(trend.End300) &&
		trend.Start100.LessThan(trend.End100) &&
		trend.Start25.LessThan(trend.End25)
}

func (trend Trend) Down() bool {
	return !trend.Up()
}

func (trend Trend) String() string {
	data, err := json.Marshal(trend)
	if err != nil {
		log.Printf("marshal trend err: %v\n", err)
	}
	return string(data)
}

func (trend Trend) Sign() string {
	var sign string
	if trend.Up() {
		sign = "Up"
	} else {
		sign = "Down"
	}
	return sign
}

func (sj *SpotJob) trend() Trend {
	now := time.Now()
	from, to := now.Add(-time.Hour*5).Unix(), now.Unix()
	result300, _, err := sj.client.SpotApi.ListCandlesticks(sj.ctx, sj.CurrencyPair.Id, &gateapi.ListCandlesticksOpts{
		From:     optional.NewInt64(from),
		To:       optional.NewInt64(to),
		Interval: optional.NewString("5m"),
		Limit:    optional.NewInt32(60),
	})
	if err != nil {
		log.Printf("refreshMarket list candle sticks err: %v\n", err)
		return Trend{}
	}
	result100, result25 := result300[41:], result300[56:]

	start300, _ := decimal.NewFromString(result300[0][2])
	end300, _ := decimal.NewFromString(result300[len(result300)-1][2])
	start100, _ := decimal.NewFromString(result100[0][2])
	end100, _ := decimal.NewFromString(result100[len(result100)-1][2])
	start25, _ := decimal.NewFromString(result25[0][2])
	end25, _ := decimal.NewFromString(result25[len(result25)-1][2])
	trend := Trend{Start300: start300, End300: end300, Start100: start100, End100: end100, Start25: start25, End25: end25}
	if trend.Up() {
		sj.Up++
	} else {
		sj.Down++
	}
	return trend
}

func (sj *SpotJob) refreshMarket() {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			log.Printf("refreshMarket panic err: %v", panicErr)
		}
	}()
	time.Sleep(time.Duration(60+sj.getRandomSecond(10)) * time.Second)
	sj.mux.Lock()
	defer sj.mux.Unlock()

	trend := sj.trend()
	buyOrders, sellOrders, _ := sj.currentOrders()
	// _, _, bidPrice, _, _ := sj.lookupMarketPrice()
	if len(buyOrders) > 0 {
		if trend.Up() {
			_, _, err := sj.client.SpotApi.CancelOrder(sj.ctx, buyOrders[0].Id, sj.CurrencyPair.Id, &gateapi.CancelOrderOpts{})
			if err != nil {
				log.Printf("refreshMarket cancel order err: %v", err)
			}
		} else if trend.Down() {
			if len(sellOrders) > 0 {
				sellPrice, _ := decimal.NewFromString(sellOrders[0].Price)
				cancelOrder := buyOrders[len(buyOrders)-1]
				cancelPrice, _ := decimal.NewFromString(cancelOrder.Price)
				if distanceRate := cancelPrice.DivRound(sellPrice, 2); distanceRate.GreaterThan(decimal.NewFromFloat(0.95)) {
					_, _, err := sj.client.SpotApi.CancelOrder(sj.ctx, cancelOrder.Id, sj.CurrencyPair.Id, &gateapi.CancelOrderOpts{})
					if err != nil {
						log.Printf("refreshMarket cancel order err: %v", err)
					}
				}
			}
		}
	}
}

func (sj *SpotJob) refreshOrderBook() {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			log.Printf("refreshOrderBook panic err: %v", panicErr)
		}
	}()
	time.Sleep(time.Duration(60+sj.getRandomSecond(10)) * time.Second)
	askPrice, _, bidPrice, _, err := sj.lookupMarketPrice()
	if err != nil {
		sj.State[2] = false
		return
	}
	sj.State[2] = true
	trend := sj.trend()
	log.Printf("refreshOrderBook -[%s], market - up:%d, down:%d", sj.CurrencyPair.Base, sj.Up, sj.Down)
	_, _, openOrders := sj.currentOrders()
	fmt.Printf("\n\n")
	log.Printf("%s\n", strings.Repeat("*", 185))
	log.Printf("%s [ %4s ] %s\n", strings.Repeat("*", 88), trend.Sign(), strings.Repeat("*", 88))
	log.Printf("%s %-10s - Ask :::::::::::::::::: - Market - :::::::::::::::::: Bid - %10s %s\n", strings.Repeat("*", 51), askPrice, bidPrice, strings.Repeat("*", 51))
	log.Printf("%s\n", strings.Repeat("*", 185))
	log.Printf("| %20s | %20s | %20s | %20s | %20s | %20s | %20s | %20s |\n", "CURRENCY", "ID", "TEXT", "SIDE", "PRICE", "AMOUNT", "LEFT", "TIME")
	log.Printf("%s\n", strings.Repeat("*", 185))
	for i, order := range openOrders {
		log.Printf("| %20s | %20s | %20s | %20s | %20s | %20s | %20s | %20s |\n", sj.CurrencyPair.Base, order.Id, order.Text, order.Side, order.Price, order.Amount, order.Left, time.UnixMilli(order.CreateTimeMs).Format("2006-01-02 15:04:05"))
		if i < len(openOrders)-1 {
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
	sj.socketMux.Lock()
	defer sj.socketMux.Unlock()
	t := time.Now().Unix()
	ordersMsg := channel.NewMsg("spot.orders", "subscribe", t, []string{sj.CurrencyPair.Id})
	ordersMsg.Sign(sj.Key, sj.Secret)
	if err := ordersMsg.Send(sj.socket); err != nil {
		panic(err)
	}
}

func (sj *SpotJob) unsubscribe() {
	sj.socketMux.Lock()
	defer sj.socketMux.Unlock()
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
			sj.Ping()
		case <-sj.ctx.Done():
			return
		}
	}
}

func (sj *SpotJob) Ping() {
	sj.socketMux.Lock()
	defer sj.socketMux.Unlock()
	t := time.Now().Unix()
	pingMsg := channel.NewMsg(channel.SpotChannelPing, "", t, []string{})
	if err := pingMsg.Ping(sj.socket); err != nil {
		sj.State[0] = false
		log.Printf("job [%s] ping err %v\n", sj.CurrencyPair.Base, err)
		return
	}
	sj.State[0] = true
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

func (sj *SpotJob) currentOrders() ([]gateapi.Order, []gateapi.Order, []gateapi.Order) {
	openOrders, _, err := sj.client.SpotApi.ListOrders(sj.ctx, sj.CurrencyPair.Id, channel.SpotChannelOrdersStatusOpen, &gateapi.ListOrdersOpts{})
	if err != nil {
		sj.State[2] = false
		log.Printf("handlePutEvent sj.client.SpotApi.ListOrders err: %v\n", err)
		return make([]gateapi.Order, 0), make([]gateapi.Order, 0), make([]gateapi.Order, 0)
	}
	sj.State[2] = true
	buyOrders, sellOrders := make([]gateapi.Order, 0), make([]gateapi.Order, 0)
	for _, order := range openOrders {
		if order.Side == channel.SpotChannelOrderSideBuy {
			buyOrders = append(buyOrders, order)
		} else if order.Side == channel.SpotChannelOrderSideSell {
			sellOrders = append(sellOrders, order)
		}
	}
	sort.Slice(buyOrders, func(i, j int) bool {
		leftPrice, _ := decimal.NewFromString(buyOrders[i].Price)
		rightPrice, _ := decimal.NewFromString(buyOrders[j].Price)
		return leftPrice.LessThanOrEqual(rightPrice)
	})
	sort.Slice(sellOrders, func(i, j int) bool {
		leftPrice, _ := decimal.NewFromString(sellOrders[i].Price)
		rightPrice, _ := decimal.NewFromString(sellOrders[j].Price)
		return leftPrice.LessThanOrEqual(rightPrice)
	})
	return buyOrders, sellOrders, openOrders
}

func (sj *SpotJob) handleOrderPutEvent(order *channel.Order) {
	log.Printf("A new order [%s]-[%s]-[%s] was put, [price: %s, amount: %s]\n", order.Text, sj.CurrencyPair.Base, order.Side, order.Price, order.Amount)
	sj.refreshOrders()
}

func (sj *SpotJob) getRandomSecond(base uint32) uint32 {
	return base % rand.Uint32()
}

func (sj *SpotJob) refreshOrders() {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			log.Printf("refreshOrders panic err: %v", panicErr)
		}
	}()
	time.Sleep(time.Duration(10+sj.getRandomSecond(10)) * time.Second)
	sj.mux.Lock()
	defer sj.mux.Unlock()

	trend := sj.trend()
	askPrice, _, bidPrice, _, err := sj.lookupMarketPrice()
	if err != nil {
		sj.State[2] = false
		return
	}
	rate := sj.Gap
	nextRate := decimal.NewFromInt(1).Sub(rate).RoundUp(3)
	if trend.Up() {
		nextRate = decimal.NewFromInt(1)
	}
	nextOrderPrice := decimal.Min(askPrice, bidPrice).Mul(nextRate).RoundFloor(sj.CurrencyPair.Precision)

	// choose a better oder price
	buyOrders, sellOrders, _ := sj.currentOrders()
	if len(buyOrders) > 0 {
		topPrice, _ := decimal.NewFromString(buyOrders[len(buyOrders)-1].Price)
		bottomPrice, _ := decimal.NewFromString(buyOrders[0].Price)
		if nextTopPrice := topPrice.Mul(decimal.NewFromFloat(1).Add(rate)).RoundFloor(sj.CurrencyPair.Precision); nextTopPrice.LessThanOrEqual(nextOrderPrice) && trend.Up() {
			nextOrderPrice = nextTopPrice
		} else {
			nextBottomPrice := bottomPrice.Mul(decimal.NewFromFloat(1).Sub(rate)).RoundFloor(sj.CurrencyPair.Precision)
			nextOrderPrice = nextBottomPrice
		}
		if (trend.Up() && nextOrderPrice.LessThanOrEqual(topPrice)) || (trend.Down() && nextOrderPrice.GreaterThanOrEqual(bottomPrice)) {
			log.Printf("refreshOrders trend - [%v], next order price - [%v], top order price - [%v], bottom order price - [%v]\n", nextOrderPrice, topPrice, bottomPrice, trend.Sign())
			return
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

	var distance = decimal.NewFromFloat(1)
	if len(sellOrders) > 0 {
		bottomSellOrderPrice, _ := decimal.NewFromString(sellOrders[0].Price)
		nextSellOrderPrice := nextOrderPrice.Mul(decimal.NewFromFloat(1).Add(sj.Gap.Mul(decimal.NewFromFloat(3))))
		distance = bottomSellOrderPrice.Sub(nextSellOrderPrice)
		log.Printf("[%s] refresh orders, bottomSellOrderPrice[%v] - nextSellOrderPrice[%v] = distance[%v]", sj.CurrencyPair.Base, bottomSellOrderPrice, nextSellOrderPrice, distance)
	}
	if len(buyOrders) < 10 && distance.GreaterThan(decimal.Zero) {
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
	}
	sj.State[2] = true
}

func (sj *SpotJob) handleOrderFinishEvent(order *channel.Order) {
	if order.Left.GreaterThan(decimal.Zero) {
		log.Printf("Order [%s]-[%s] was cancelled, [price: %s, amount: %s/%s]\n", sj.CurrencyPair.Base, order.Text, order.Price, order.Left, order.Amount)
		return
	}
	log.Printf("Order [%s]-[%s] was closed, [price: %s, amount: %s, fee: %s/%s]\n", sj.CurrencyPair.Base, order.Text, order.Price, order.Amount, order.Fee, order.FeeCurrency)
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
