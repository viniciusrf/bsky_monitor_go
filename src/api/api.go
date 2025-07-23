package api

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

func StartAPI() error {
	apiPort := 8080
	if portStr := os.Getenv("API_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			apiPort = p
		}
	}

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	router.GET("/monitor", handleGetFeed)

	fmt.Printf("Starting API server on port %d\n", apiPort)
	return router.Run(fmt.Sprintf(":%d", apiPort))
}
