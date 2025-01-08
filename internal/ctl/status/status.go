package status

import (
	"github.com/gin-gonic/gin"
	"github.com/pikvm/kvmd-cloud/internal/ctl"
)

func SetupRoutes(r *gin.Engine) {
	r.GET("/status", getStatus)
}

func getStatus(c *gin.Context) {
	c.JSON(200, ctl.ApplicationStatusResponse{
		PingerField: "Yahoo!!",
	})
}
