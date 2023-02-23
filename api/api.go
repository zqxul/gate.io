package api

import "github.com/gin-gonic/gin"

var R *gin.Engine = gin.Default()

func init() {
	R.GET("/ping", HandlePing)
	R.Run()
}

func HandlePing(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}
