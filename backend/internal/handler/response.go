package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Response{Code: code, Message: message, Data: nil})
}

func FailWithField(c *gin.Context, httpStatus, code int, message string, fields map[string]string) {
	c.JSON(httpStatus, Response{Code: code, Message: message, Data: gin.H{"fields": fields}})
}

func ClientIP(c *gin.Context) string {
	return c.ClientIP()
}
