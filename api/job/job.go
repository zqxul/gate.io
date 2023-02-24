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
	State        [3]bool              `json:"state"`
}

func init() {
	jobGroup := api.R.Group("/job")
	jobGroup.GET("/list", HandleListJobs)
	jobGroup.GET("/:id", HandleGetJob)
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
		})
	}
	c.JSON(http.StatusOK, results)
}

func HandleGetJob(c *gin.Context) {
	ID := c.Param("id")
	if result := job.GetJob(ID); result != nil {
		c.JSON(http.StatusOK, result)
		return
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
	if exist := job.EditJob(ID, jobInfo.Gap, jobInfo.OrderAmount, jobInfo.Fund, jobInfo.OrderNum); exist {
		c.JSON(http.StatusOK, nil)
		return
	}
	c.JSON(http.StatusNotFound, nil)
}

func HandleStopJob(c *gin.Context) {
	ID := c.Param("id")
	if exist := job.StopJob(ID); exist {
		c.JSON(http.StatusOK, nil)
		return
	}
	c.JSON(http.StatusNotFound, nil)
}

func HandleResumeJob(c *gin.Context) {
	ID := c.Param("id")
	if exist := job.ResumeJob(ID); exist {
		c.JSON(http.StatusOK, nil)
		return
	}
	c.JSON(http.StatusNotFound, nil)
}

func HandleDeleteJob(c *gin.Context) {
	ID := c.Param("id")
	if exist := job.RemoveJob(ID); exist {
		c.JSON(http.StatusOK, nil)
		return
	}
	c.JSON(http.StatusNotFound, nil)
}
