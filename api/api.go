package api

import (
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

var e *gin.Engine = gin.Default()
var R gin.RouterGroup = *e.Group("/gateio")

var Empty = make(map[string]interface{}, 0)

type Err struct {
	Err error `json:"error"`
}

func init() {
	R.GET("/ping", HandlePing)
}

func Run() {
	pprof.Register(e)
	pprof.RouteRegister(&R, "pprof")
	e.Run(":8888")
}

func HandlePing(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}
