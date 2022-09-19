package api

import (
	// internal packages
	http "net/http"
	time "time"
	// external packages
	cors "github.com/gin-contrib/cors"
	gzip "github.com/gin-contrib/gzip"
	gin "github.com/gin-gonic/gin"
)

////////////////////
//     router     //
////////////////////

func GetAPIRouter() http.Handler {
	// router instance
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// cors middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Accept", "Cache-Control", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Authorization", "Content-Type"},
		MaxAge:           12 * time.Hour,
		AllowCredentials: true,
	}))

	// default compression middleware
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	// routes
	apiRouter := router.Group("/api")
	{
		apiRouter.POST("/whatsapp/send", sendMessage)
		apiRouter.GET("/connect/qr", getConnectionQRCode)
	}

	// return router
	return router
}
