package ctlserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pikvm/kvmd-cloud/internal/ctl"
	"github.com/pikvm/kvmd-cloud/internal/hive"
)

func certbotAdd(c *gin.Context) {
	var request ctl.CertbotDomainName
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ctl.CertbotResponse{
			Ok:    false,
			Error: err.Error(),
		})
	}
	if err := hive.CertbotAdd(c.Request.Context(), request.DomainName, request.TXT); err != nil {
		c.JSON(http.StatusBadRequest, ctl.CertbotResponse{
			Ok:    false,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ctl.CertbotResponse{
		Ok:    true,
		Error: "",
	})
}

func certbotDel(c *gin.Context) {
	var request ctl.CertbotDomainName
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ctl.CertbotResponse{
			Ok:    false,
			Error: err.Error(),
		})
	}
	if err := hive.CertbotDel(c.Request.Context(), request.DomainName); err != nil {
		c.JSON(http.StatusBadRequest, ctl.CertbotResponse{
			Ok:    false,
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ctl.CertbotResponse{
		Ok:    true,
		Error: "",
	})
}
