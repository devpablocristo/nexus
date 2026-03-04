package ginserver

import (
	"github.com/gin-gonic/gin"
)

type EngineOptions struct {
	Mode string
}

func NewEngine(opts EngineOptions, middleware ...gin.HandlerFunc) *gin.Engine {
	if opts.Mode != "" {
		gin.SetMode(opts.Mode)
	}
	r := gin.New()
	for _, mw := range middleware {
		r.Use(mw)
	}
	return r
}
