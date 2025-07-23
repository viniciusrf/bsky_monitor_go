package api

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

func StartAPI() error {
	if apiPort := os.Getenv("API_PORT"); apiPort != "" {
		apiPort = "8080"
	}

	router := gin.Default()

	// Basic health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Account routes
	carsData := router.Group("/monitor")
	{
		carsData.GET("/:car/download-repo", handleDownloadRepo)
		carsData.GET("/:car/list-records", handleListRecords)
		carsData.POST("/:car/list-blobs", handleListBlobs)
	}

	// Monitoring endpoint
	router.POST("/monitor", handleMonitorMedia)

	fmt.Printf("Starting API server on port %d\n", apiPort)
	return r.Run(fmt.Sprintf(":%d", apiPort))
}
