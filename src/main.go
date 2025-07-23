package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	bsky "github.com/bluesky-social/indigo/api/bsky"
	_ "github.com/bluesky-social/indigo/api/chat"
	_ "github.com/bluesky-social/indigo/api/ozone"
	"github.com/bluesky-social/indigo/xrpc"

	api "github.com/viniciusrf/bsky_monitor_go/src/api"
	auth "github.com/viniciusrf/bsky_monitor_go/src/auth"
	feed "github.com/viniciusrf/bsky_monitor_go/src/feed"
	media "github.com/viniciusrf/bsky_monitor_go/src/media"
)

var (
	account  string
	feedType string
	user     string
)

// implementing cursor position

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {

	if len(os.Args) != 3 {
		account = os.Getenv("BSKY_ACCOUNT")
		user = os.Getenv("BSKY_USER")
		feedType = os.Getenv("BSKY_FILTER")
	} else {
		account = os.Args[2]
	}

	switch os.Args[1] {
	case "download-repo":
		return carDownload(account)
	case "list-records":
		return carList(account)
	case "unpack-records":
		return carUnpack(account)
	case "list-blobs":
		return media.BlobList(account)
	case "download-blobs":
		return media.BlobDownloadAll(account)
	case "monitor_media":
		return monitorAccMedia(account, feedType)
	case "apiServer":
		return api.StartAPI()
	default:
		return fmt.Errorf("unexpected command: %s", os.Args[1])
	}
}

func monitorAccMedia(account, feedType string) error {
	ctx := context.Background()
	ident, err := auth.ParseUserHandle(account)
	if err != nil {
		return err
	}
	xrpcc := xrpc.Client{
		Host: ident.PDSEndpoint(),
	}
	if xrpcc.Host == "" {
		return fmt.Errorf("no PDS endpoint for identity")
	}

	//LOGIN
	xrpcc, err = auth.KeepAlive(ctx, xrpcc)
	if err != nil {
		return fmt.Errorf("failed to execute login: %v", err)
	}

	runningMonitor := true
	processedIDsFile := "processed_ids.txt"
	cursor := ""
	fmt.Printf("Monitor started at %s\n", time.Now().Format(time.RFC1123))

	var wg sync.WaitGroup
	maxConcurrent := 5
	semaphore := make(chan struct{}, maxConcurrent)

	for runningMonitor {

		processedIDs, err := ReadProcessedIDs(processedIDsFile)
		if err != nil {
			return fmt.Errorf("failed to read processed IDs: %v", err)
		}

		xrpcc, err = auth.KeepAlive(ctx, xrpcc)
		if err != nil {
			return fmt.Errorf("failed to execute login: %v", err)
		}
		responseFeed, err := feed.GetFeed(ctx, &xrpcc, feedType, cursor, account)
		if err != nil {
			return err
		}
		for _, post := range responseFeed.Feed {
			if processedIDs[post.Post.Cid] {
				continue
			}
			if feedType == "nsfw" && !feed.CheckLabelsNSFW(post.Post.Labels) {
				IdProcessed(processedIDsFile, post.Post.Cid)
				continue
			}
			if post.Post.Embed != nil {
				wg.Add(1)
				semaphore <- struct{}{}
				go func(p *bsky.FeedDefs_FeedViewPost, acc string) {
					defer wg.Done()
					defer func() { <-semaphore }()

					media.EmbedResolve(p, acc)

					IdProcessed(processedIDsFile, p.Post.Cid)
				}(post, account)
			}
			IdProcessed(processedIDsFile, post.Post.Cid)
		}
		fmt.Printf("Last run at %s\n", time.Now().Format(time.RFC1123))
		time.Sleep(2 * time.Minute)
	}

	return nil
}

func IdProcessed(processedIDsFile, postCid string) {
	err := WriteProcessedID(processedIDsFile, postCid)
	if err != nil {
		fmt.Printf("failed to write processed ID: %v", err)
	}
}

func ReadProcessedIDs(filename string) (map[string]bool, error) {
	processedIDs := make(map[string]bool)

	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return processedIDs, nil
		}
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		processedIDs[scanner.Text()] = true
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	return processedIDs, nil
}

// WriteProcessedID appends a new ID to the file.
func WriteProcessedID(filename, id string) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	_, err = file.WriteString(id + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to file: %v", err)
	}

	return nil
}
