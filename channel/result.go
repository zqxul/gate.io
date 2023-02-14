package channel

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

type OrderResult []Order

type GateMessage struct {
	Time    int64       `json:"time"`
	TimeMS  int64       `json:"time_ms"`
	Channel string      `json:"channel"`
	Event   string      `json:"event"`
	Result  interface{} `json:"result"`
}

func (m *GateMessage) ParseResult() string {
	r, err := json.Marshal(m.Result)
	if err != nil {
		panic(err)
	}
	return string(r)
}

type Ticker struct {
	CurrencyPair     string          `json:"currency_pair"`
	Last             decimal.Decimal `json:"last"`
	LowestAsk        decimal.Decimal `json:"lowest_ask"`
	HighestBid       decimal.Decimal `json:"highest_bid"`
	ChangePercentage decimal.Decimal `json:"change_percentage"`
	BaseVolumn       decimal.Decimal `json:"base_volume"`
	QuotoVolumn      decimal.Decimal `json:"quote_volume"`
	High24H          decimal.Decimal `json:"high_24h"`
	Low24H           decimal.Decimal `json:"low_24h"`
}

type Order struct {
	Id                 string          `json:"id,omitempty"`                   // Order ID
	User               int64           `json:"user"`                           // User ID
	Text               string          `json:"text,omitempty"`                 // User defined information. If not empty, must follow the rules below:  1. prefixed with `t-` 2. no longer than 28 bytes without `t-` prefix 3. can only include 0-9, A-Z, a-z, underscore(_), hyphen(-) or dot(.)
	CreateTime         string          `json:"create_time,omitempty"`          // Order creation time
	CreateTimeMS       string          `json:"create_time_ms,omitempty"`       // Order creation time in milliseconds
	UpdateTime         string          `json:"update_time,omitempty"`          // Order last modification time
	UpdateTimeMS       string          `json:"update_time_ms,omitempty"`       // Order last modification time in milliseconds
	Event              string          `json:"event,omitempty"`                // Order event - put: order creation - update: order fill update - finish: order closed or cancelled
	CurrencyPair       string          `json:"currency_pair"`                  // Currency pair
	Type               string          `json:"type,omitempty"`                 // Order type. limit - limit order
	Account            string          `json:"account,omitempty"`              // Account type. spot - spot account; margin - margin account
	Side               string          `json:"side"`                           // Order side. -buy; -sell
	Amount             decimal.Decimal `json:"amount"`                         // Trade amount
	Price              decimal.Decimal `json:"price"`                          // Order price
	TimeInForce        string          `json:"time_in_force,omitempty"`        // Time in force  - gtc: GoodTillCancelled - ioc: ImmediateOrCancelled, taker only - poc: PendingOrCancelled, makes a post-only order that always enjoys a maker fee
	Left               decimal.Decimal `json:"left,omitempty"`                 // Amount left to fill
	FilledTotal        decimal.Decimal `json:"filled_total,omitempty"`         // Total filled in quote currency
	AvgDealPrice       decimal.Decimal `json:"avg_deal_price"`                 // Average transaction price of orders
	Fee                decimal.Decimal `json:"fee,omitempty"`                  // Fee deducted
	FeeCurrency        string          `json:"fee_currency,omitempty"`         // Fee currency unit
	PointFee           decimal.Decimal `json:"point_fee,omitempty"`            // Point used to deduct fee
	GtFee              decimal.Decimal `json:"gt_fee,omitempty"`               // GT used to deduct fee
	GtDiscount         bool            `json:"gt_discount,omitempty"`          // Whether GT fee discount is used
	RebatedFee         decimal.Decimal `json:"rebated_fee,omitempty"`          // Rebated fee
	RebatedFeeCurrency string          `json:"rebated_fee_currency,omitempty"` // Rebated fee currency unit
}

type OrderBook struct {
	ID      int64             `json:"id"`
	Current int64             `json:"current"`
	Update  int64             `json:"update"`
	Asks    []decimal.Decimal `json:"asks"`
	Bids    []decimal.Decimal `json:"bids"`
}
