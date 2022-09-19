package api

import (
	// internal packages
	http "net/http"
	// external packages
	gin "github.com/gin-gonic/gin"
	// local packages
	bridge "bitaminco/support-whatsapp-bridge/src/bridge"
	types "bitaminco/support-whatsapp-bridge/src/types"
)

//////////////////
//    routes    //
//////////////////

// send whatsapp message
func sendMessage(c *gin.Context) {
	// extract details
	var formData types.ST_Form_SendMessage
	err := c.ShouldBind(&formData)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// send whatsapp message
	responseString := *bridge.SendWhatsappMessage(
		&formData.FromPhone, &formData.ToPhone, &formData.MessageText,
	)
	if responseString != "" {
		// on error
		c.JSON(http.StatusOK, gin.H{
			"info": responseString,
			"data": nil,
		})
		return
	}

	// message sent
	c.JSON(http.StatusOK, gin.H{
		"info": "Message sent!",
		"data": nil,
	})
}
