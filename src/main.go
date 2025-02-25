package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	bsky "github.com/bluesky-social/indigo/api/bsky"
	_ "github.com/bluesky-social/indigo/api/chat"
	_ "github.com/bluesky-social/indigo/api/ozone"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	auth "github.com/viniciusrf/bsky_monitor_go/src/auth"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	account := ""
	if len(os.Args) != 3 {
		account = os.Getenv("BSKY_ACCOUNT")
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
		return blobList(account)
	case "download-blobs":
		return blobDownloadAll(account)
	case "monitor_media":
		return monitorAccMedia(account)
	default:
		return fmt.Errorf("unexpected command: %s", os.Args[1])
	}
}

func parseUserHandle(raw string) (*identity.Identity, error) {
	ctx := context.Background()
	atid, err := syntax.ParseAtIdentifier(raw)
	if err != nil {
		return nil, fmt.Errorf("Failed to get ParseAtIdentifier: %v", err)
	}

	// first look up the DID and PDS for this repo
	dir := identity.DefaultDirectory()
	ident, err := dir.Lookup(ctx, *atid)
	if err != nil {
		return nil, fmt.Errorf("Failed to get DefaultDirectory-Lookup: %v", err)
	}
	return ident, nil
}

func blobList(raw string) error {
	ctx := context.Background()
	ident, err := parseUserHandle(raw)
	if err != nil {
		return err
	}

	// create a new API client to connect to the account's PDS
	xrpcc := xrpc.Client{
		Host: ident.PDSEndpoint(),
	}
	if xrpcc.Host == "" {
		return fmt.Errorf("no PDS endpoint for identity")
	}

	cursor := ""
	for {
		resp, err := comatproto.SyncListBlobs(ctx, &xrpcc, cursor, ident.DID.String(), 500, "")
		if err != nil {
			return err
		}
		for _, cidStr := range resp.Cids {
			fmt.Println(cidStr)
		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}
	return nil
}

func blobDownloadAll(raw string) error {
	ctx := context.Background()
	ident, err := parseUserHandle(raw)
	if err != nil {
		return err
	}

	// create a new API client to connect to the account's PDS
	xrpcc := xrpc.Client{
		Host: ident.PDSEndpoint(),
	}
	if xrpcc.Host == "" {
		return fmt.Errorf("no PDS endpoint for identity")
	}

	topDir := ident.DID.String() + "/_blob"
	fmt.Printf("writing blobs to: %s\n", topDir)
	os.MkdirAll(topDir, os.ModePerm)

	cursor := ""
	for {
		resp, err := comatproto.SyncListBlobs(ctx, &xrpcc, cursor, ident.DID.String(), 500, "")
		if err != nil {
			return err
		}
		for _, cidStr := range resp.Cids {
			blobPath := topDir + "/" + cidStr
			if _, err := os.Stat(blobPath); err == nil {
				fmt.Printf("%s\texists\n", blobPath)
				continue
			}
			blobBytes, err := comatproto.SyncGetBlob(ctx, &xrpcc, cidStr, ident.DID.String())
			if err != nil {
				return err
			}
			if err := os.WriteFile(blobPath, blobBytes, 0666); err != nil {
				return err
			}
			fmt.Printf("%s\tdownloaded\n", blobPath)
		}
		if resp.Cursor != nil && *resp.Cursor != "" {
			cursor = *resp.Cursor
		} else {
			break
		}
	}
	return nil
}

func monitorAccMedia(raw string) error {
	ctx := context.Background()
	ident, err := parseUserHandle(raw)
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

	for runningMonitor {
		fmt.Printf("Monitor started at %s\n", time.Now().Format(time.RFC1123))
		//GET 3 LAST MESSAGES
		processedIDs, err := ReadProcessedIDs(processedIDsFile)
		if err != nil {
			return fmt.Errorf("failed to read processed IDs: %v", err)
		}

		xrpcc, err = auth.KeepAlive(ctx, xrpcc)
		if err != nil {
			return fmt.Errorf("failed to execute login: %v", err)
		}
		cursor := ""
		resp, err := bsky.FeedGetAuthorFeed(ctx, &xrpcc, raw, cursor, "posts_with_media", false, 5)
		if err != nil {
			return err
		}
		for _, post := range resp.Feed {
			if processedIDs[post.Post.Cid] {
				continue
			}
			if post.Post.Embed != nil {
				embed := post.Post.Embed
				if embed.EmbedImages_View != nil {
					getImages(embed.EmbedImages_View.Images, post.Post.Cid, raw)
				}
				if embed.EmbedVideo_View != nil {
					fmt.Printf("Video Found")
				}
				if embed.EmbedExternal_View != nil {
					fmt.Printf("External Found")
				}
				if embed.EmbedRecordWithMedia_View != nil {
					if embed.EmbedRecordWithMedia_View.Media.EmbedImages_View != nil {
						getImages(embed.EmbedRecordWithMedia_View.Media.EmbedImages_View.Images, post.Post.Cid, raw)
					}
				}
			}
			err = WriteProcessedID(processedIDsFile, post.Post.Cid)
			if err != nil {
				return fmt.Errorf("failed to write processed ID: %v", err)
			}
		}
		time.Sleep(2 * time.Minute)
	}

	return nil
}

func getImages(images []*bsky.EmbedImages_ViewImage, ID string, raw string) error {
	for counter, image := range images {
		folder := "files/" + raw
		if err := os.MkdirAll(folder, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create folder: %v", err)
		}

		resp, err := http.Get(image.Fullsize)
		if err != nil {
			return fmt.Errorf("failed to download image: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download image: status code %d", resp.StatusCode)
		}

		filePath := filepath.Join(folder, fmt.Sprintf("%s-%s-%03d.jpg", strings.ReplaceAll(image.Alt, " ", "-"), ID, counter))

		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %v", err)
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to save image: %v", err)
		}

		fmt.Printf("Image saved: %s\n", filePath)
	}
	return nil
}

// ReadProcessedIDs reads the processed post IDs from a file.
func ReadProcessedIDs(filename string) (map[string]bool, error) {
	processedIDs := make(map[string]bool)

	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// If the file doesn't exist, return an empty map
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
