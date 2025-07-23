package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/gin-gonic/gin"
	"github.com/viniciusrf/bsky_monitor_go/src/auth"
	"github.com/viniciusrf/bsky_monitor_go/src/feed"
)

func handleGetFeed(c *gin.Context) {
	account := c.Query("account")
	feedType := c.Query("feed_type")

	ctx := context.Background()
	ident, err := auth.ParseUserHandle(account)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	xrpcc := xrpc.Client{
		Host: ident.PDSEndpoint(),
	}
	if xrpcc.Host == "" {
		c.JSON(500, gin.H{"error": err.Error()})
	}

	//LOGIN
	xrpcc, err = auth.KeepAlive(ctx, xrpcc)
	if err != nil {
		c.JSON(500, gin.H{"Failed to execute Login - error": err.Error()})
	}

	feed, err := feed.GetFeed(ctx, &xrpcc, feedType, "", account)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	jsonData, err := json.Marshal(feed)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", jsonData)
}
