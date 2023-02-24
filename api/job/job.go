package job

import (
	"fmt"
	"net/http"

	"gate.io/api"
	"gate.io/channel"
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
	State        [3]bool              `json:"state"`
	Stoped       bool                 `json:"stoped"`
}

func (ji *JobInfo) Validate() error {
	if ji.Gap.LessThanOrEqual(decimal.NewFromFloat(0.0001)) {
		return fmt.Errorf("invalid gap value")
	}
	return nil
}

func init() {
	jobGroup := api.R.Group("/job")
	jobGroup.GET("/list", HandleListJobs)
	jobGroup.GET("/:id", HandleGetJob)
	jobGroup.POST("", HandleNewJob)
	jobGroup.POST("/:id", HandleEditJob)
	jobGroup.POST("/:id/stop", HandleStopJob)
	jobGroup.POST("/:id/resume", HandleResumeJob)
	jobGroup.DELETE("':id", HandleDeleteJob)
}

func HandleListJobs(c *gin.Context) {
	results := make([]JobInfo, 0)
	for _, job := range job.List() {
		results = append(results, JobInfo{
			CurrencyPair: job.CurrencyPair,
			Gap:          job.Gap,
			OrderAmount:  job.OrderAmount,
			OrderNum:     job.OrderNum,
			Fund:         job.Fund,
			State:        job.State,
			Stoped:       job.Stoped,
		})
	}
	c.JSON(http.StatusOK, results)
}

func HandleNewJob(c *gin.Context) {
	jobInfo := JobInfo{}
	if err := c.ShouldBindJSON(&jobInfo); err != nil {
		c.JSON(http.StatusBadRequest, api.Empty)
		return
	}
	if err := jobInfo.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	result := job.New(jobInfo.CurrencyPair.Id, jobInfo.Fund, jobInfo.Gap, channel.SecondKey, channel.SecondSecret)
	c.JSON(http.StatusOK, result)
}

func HandleGetJob(c *gin.Context) {
	ID := c.Param("id")
	if result := job.Get(ID); result != nil {
		c.JSON(http.StatusOK, JobInfo{
			CurrencyPair: result.CurrencyPair,
			Gap:          result.Gap,
			OrderAmount:  result.OrderAmount,
			OrderNum:     result.OrderNum,
			Fund:         result.Fund,
			State:        result.State,
			Stoped:       result.Stoped,
		})
		return
	}
	c.JSON(http.StatusNotFound, api.Empty)
}

func HandleEditJob(c *gin.Context) {
	ID := c.Param("id")
	jobInfo := JobInfo{}
	if err := c.ShouldBindJSON(&jobInfo); err != nil {
		c.JSON(http.StatusBadRequest, api.Empty)
		return
	}
	if err := jobInfo.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	if exist := job.Edit(ID, jobInfo.Gap, jobInfo.OrderAmount, jobInfo.Fund, jobInfo.OrderNum); exist {
		c.JSON(http.StatusOK, api.Empty)
		return
	}
	c.JSON(http.StatusNotFound, api.Empty)
}

func HandleStopJob(c *gin.Context) {
	ID := c.Param("id")
	if exist := job.Stop(ID); exist {
		c.JSON(http.StatusOK, api.Empty)
		return
	}
	c.JSON(http.StatusNotFound, api.Empty)
}

func HandleResumeJob(c *gin.Context) {
	ID := c.Param("id")
	if exist := job.Resume(ID); exist {
		c.JSON(http.StatusOK, api.Empty)
		return
	}
	c.JSON(http.StatusNotFound, api.Empty)
}

func HandleDeleteJob(c *gin.Context) {
	ID := c.Param("id")
	if exist := job.Remove(ID); exist {
		c.JSON(http.StatusOK, api.Empty)
		return
	}
	c.JSON(http.StatusNotFound, api.Empty)
}
