package job

import (
	"net/http"

	"gate.io/api"
	"gate.io/channel"
	"github.com/gateio/gateapi-go/v6"
	"github.com/gin-gonic/gin"
)

var client *gateapi.APIClient

func init() {
	cfg := gateapi.NewConfiguration()
	cfg.Key = channel.SecondKey
	cfg.Secret = channel.SecondSecret
	client = gateapi.NewAPIClient(cfg)
}

func init() {
	jobGroup := api.R.Group("/currency-pairs")
	jobGroup.GET("/list", ListCurrencyPairs)
}

func ListCurrencyPairs(c *gin.Context) {
	currencyPairs, _, err := client.SpotApi.ListCurrencyPairs(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, currencyPairs)
}
