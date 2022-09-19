package api

import (
	// internal packages
	png "image/png"
	http "net/http"
	// external packages
	barcode "github.com/boombuler/barcode"
	qr "github.com/boombuler/barcode/qr"
	gin "github.com/gin-gonic/gin"
	// local packages
	bridge "bitaminco/support-whatsapp-bridge/src/bridge"
)

//////////////////
//    routes    //
//////////////////

// send whatsapp message
func getConnectionQRCode(c *gin.Context) {
	// parse params
	fromPhone := c.Query("fromPhone")
	if fromPhone == "" {
		// invalid phone
		c.JSON(http.StatusOK, gin.H{
			"info": "Invalid phone number!",
			"data": nil,
		})
		return
	}

	// connect to client
	newQRCodeBufferString := *bridge.SyncWithGivenDevice(&fromPhone)
	// if not connected then connect
	if newQRCodeBufferString == "" {
		// already scanned device
		c.JSON(http.StatusOK, gin.H{
			"info": "This device is already connected!",
			"data": nil,
		})
		return
	}

	// generate qr code
	qrCode, _ := qr.Encode(newQRCodeBufferString, qr.L, qr.Auto)
	qrCode, _ = barcode.Scale(qrCode, 512, 512)
	// send qr code
	png.Encode(c.Writer, qrCode)
}
