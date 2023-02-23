package job

import (
	"net/http"

	"gate.io/api"
	"gate.io/job"
	"github.com/gateio/gateapi-go/v6"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type JobInfo struct {
	CurrencyPair gateapi.CurrencyPair `json:"currencyPair"`
	Gap          decimal.Decimal      `json:"gap"`
	OrderAmount  decimal.Decimal      `json:"orderAmount"`
	OrderNum     int                  `json:"orderNum"`
	Fund         decimal.Decimal      `json:"fund"`
}

func init() {
	jobGroup := api.R.Group("/job")
	jobGroup.GET("/list", HandleListJobs)
	jobGroup.GET("/:id", HandleGetJob)
	jobGroup.POST("/:id", HandleEditJob)
	jobGroup.DELETE("':id")
}

func HandleListJobs(c *gin.Context) {
	jobInfos := make([]JobInfo, 0)
	for _, job := range job.List {
		jobInfos = append(jobInfos, JobInfo{
			CurrencyPair: job.CurrencyPair,
			Gap:          job.Gap,
			OrderAmount:  job.OrderAmount,
			OrderNum:     job.OrderNum,
			Fund:         job.Fund,
		})
	}
	c.JSON(http.StatusOK, jobInfos)
}

func HandleGetJob(c *gin.Context) {
	ID := c.Param("id")
	for _, job := range job.List {
		if job.CurrencyPair.Id == ID {
			c.JSON(http.StatusOK, job)
			return
		}
	}
	c.JSON(http.StatusNotFound, nil)
}

func HandleEditJob(c *gin.Context) {
	ID := c.Param("id")
	jobInfo := JobInfo{}
	if err := c.ShouldBindJSON(&jobInfo); err != nil {
		c.JSON(http.StatusBadRequest, nil)
		return
	}
	for i, item := range job.List {
		if item.CurrencyPair.Id == ID {
			job.List[i].Gap = jobInfo.Gap
			job.List[i].OrderAmount = jobInfo.OrderAmount
			job.List[i].OrderNum = jobInfo.OrderNum
			job.List[i].Fund = jobInfo.Fund
			c.JSON(http.StatusOK, item)
		}
	}
	c.JSON(http.StatusNotFound, nil)
}

func HandleStopJob(c *gin.Context) {
	ID := c.Param("id")
	for i, item := range job.List {
		if item.CurrencyPair.Id == ID {
			job.List[i].Stop()
			c.JSON(http.StatusOK, nil)
			return
		}
	}
	c.JSON(http.StatusNotFound, nil)
}

func HandleResumeJob(c *gin.Context) {

}

func HandleDeleteJob(c *gin.Context) {

}
