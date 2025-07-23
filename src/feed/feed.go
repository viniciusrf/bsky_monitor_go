package feed

import (
	"context"
	"fmt"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	bsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	models "github.com/viniciusrf/bsky_monitor_go/src/models"
)

func GetFeed(ctx context.Context, xrpcc *xrpc.Client, feedType, cursor, account string) (*models.FeedResponse, error) {
	switch feedType {
	case "nsfw":
		return getNSFWFromTimeline(ctx, *xrpcc, cursor)
	case "byAccount":
		return getImageFeedFromProfile(ctx, *xrpcc, cursor, account)
	default:
		return nil, fmt.Errorf("unexpected feed parameter: %s", feedType)
	}

}
func getNSFWFromTimeline(ctx context.Context, xrpcc xrpc.Client, cursor string) (*models.FeedResponse, error) {
	responseFeed, err := bsky.FeedGetTimeline(ctx, &xrpcc, "", cursor, int64(30))
	if err != nil {
		return nil, err
	}

	return &models.FeedResponse{
		Cursor: responseFeed.Cursor,
		Feed:   responseFeed.Feed,
	}, nil
}

func getImageFeedFromProfile(ctx context.Context, xrpcc xrpc.Client, cursor, account string) (*models.FeedResponse, error) {
	responseFeed, err := bsky.FeedGetAuthorFeed(ctx, &xrpcc, account, "", "posts_with_media", false, 5)
	if err != nil {
		return nil, err
	}

	return &models.FeedResponse{
		Cursor: responseFeed.Cursor,
		Feed:   responseFeed.Feed,
	}, nil
}

func CheckLabelsNSFW(labels []*comatproto.LabelDefs_Label) bool {
	for _, label := range labels {
		if label.Val == "porn" {
			return true
		}
	}
	return false
}
