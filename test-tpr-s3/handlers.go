package main

import (
	"encoding/json"
	//	"fmt"
	"github.com/sirupsen/logrus"
	"gopkg.in/gin-gonic/gin.v1"
	"net/http"
	"time"
)

func ServiceStatus(c *gin.Context) {
	Lock.RLock()
	defer Lock.RUnlock()

	if Lock.State["status"] != "OK" {
		c.Writer.WriteHeader(http.StatusServiceUnavailable)
	}
	bs, err := json.Marshal(Lock.State["status"])
	if err != nil {
		//		TODO..do not panic; use a recovery handler
		panic(err)
	}
	c.Writer.Write(bs)
}

func Logrus() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now().UTC()
		path := c.Request.URL.Path
		c.Next()
		end := time.Now().UTC()
		latency := end.Sub(start)
		Logger.WithFields(logrus.Fields{
			"status":     c.Writer.Status(),
			"method":     c.Request.Method,
			"path":       path,
			"ip":         c.ClientIP(),
			"duration":   latency,
			"user_agent": c.Request.UserAgent(),
		}).Info()
	}
}
