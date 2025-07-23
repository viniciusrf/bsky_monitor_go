package models

import (
	bsky "github.com/bluesky-social/indigo/api/bsky"
)

type FeedResponse struct {
	Cursor *string
	Feed   []*bsky.FeedDefs_FeedViewPost
}
