package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	bsky "github.com/bluesky-social/indigo/api/bsky"
	_ "github.com/bluesky-social/indigo/api/chat"
	_ "github.com/bluesky-social/indigo/api/ozone"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
)

var lastRefresh time.Time = time.Now()

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("expected two args: <command> <target>")
	}
	switch os.Args[1] {
	case "download-repo":
		return carDownload(os.Args[2])
	case "list-records":
		return carList(os.Args[2])
	case "unpack-records":
		return carUnpack(os.Args[2])
	case "list-blobs":
		return blobList(os.Args[2])
	case "download-blobs":
		return blobDownloadAll(os.Args[2])
	case "monitor_media":
		return monitorAccMedia(os.Args[2])
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

func authLogin(ctx context.Context, xrpc *xrpc.Client) (*comatproto.ServerCreateSession_Output, error) {
	var authInfo comatproto.ServerCreateSession_Input
	authInfo.Identifier = os.Getenv("BSKY_USER")
	authInfo.Password = os.Getenv("BSKY_PASS")

	auth, err := comatproto.ServerCreateSession(ctx, xrpc, &authInfo)
	if err != nil {
		return nil, fmt.Errorf("Failed to get ServerCreateSession_Output: %v - authInfo = %v", err)
	}
	return auth, nil
}

func refreshSession(ctx context.Context, xrpc *xrpc.Client) (*comatproto.ServerRefreshSession_Output, error) {

	auth, err := comatproto.ServerRefreshSession(ctx, xrpc)
	if err != nil {
		return nil, fmt.Errorf("Failed to get ServerRefreshSession_Output: %v - authInfo = %v", err, xrpc)
	}
	return auth, nil
}

func keepAlive(ctx context.Context, xrpcc xrpc.Client) (xrpc.Client, error) {

	if xrpcc.Auth == nil {
		authData, err := authLogin(ctx, &xrpcc)
		if err != nil {
			return xrpcc, fmt.Errorf("authLogin Error %v", err)
		}
		lastRefresh = time.Now()
		xrpcc.Auth = &xrpc.AuthInfo{
			AccessJwt:  authData.AccessJwt,
			RefreshJwt: authData.RefreshJwt,
			Did:        authData.Did,
			Handle:     authData.Handle,
		}
	}

	timeNow := time.Now()
	diff := timeNow.Sub(lastRefresh)
	if diff > 1*time.Hour {
		xrpcc.Auth.AccessJwt = xrpcc.Auth.RefreshJwt
		authData, err := refreshSession(ctx, &xrpcc)
		if err != nil {
			return xrpcc, fmt.Errorf("refreshLogin Error %v", err)
		}
		lastRefresh = time.Now()
		fmt.Println("Refreshed token @ : %s ", time.Now().Format(time.RFC1123))
		xrpcc.Auth = &xrpc.AuthInfo{
			AccessJwt:  authData.AccessJwt,
			RefreshJwt: authData.RefreshJwt,
			Did:        authData.Did,
			Handle:     authData.Handle,
		}
	}

	return xrpcc, nil

}

func carDownload(raw string) error {
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

	carPath := ident.DID.String() + ".car"
	fmt.Printf("downloading from %s to: %s\n", xrpcc.Host, carPath)
	repoBytes, err := comatproto.SyncGetRepo(ctx, &xrpcc, ident.DID.String(), "")
	if err != nil {
		return err
	}
	return os.WriteFile(carPath, repoBytes, 0666)
}

func carList(carPath string) error {
	ctx := context.Background()
	fi, err := os.Open(carPath)
	if err != nil {
		return err
	}

	// read repository tree in to memory
	r, err := repo.ReadRepoFromCar(ctx, fi)
	if err != nil {
		return err
	}

	// extract DID from repo commit
	sc := r.SignedCommit()
	did, err := syntax.ParseDID(sc.Did)
	if err != nil {
		return err
	}

	fmt.Printf("=== %s ===\n", did)
	fmt.Println("key\trecord_cid")

	err = r.ForEach(ctx, "", func(k string, v cid.Cid) error {
		fmt.Printf("%s\t%s\n", k, v.String())
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func carUnpack(carPath string) error {
	ctx := context.Background()
	fi, err := os.Open(carPath)
	if err != nil {
		return err
	}

	r, err := repo.ReadRepoFromCar(ctx, fi)
	if err != nil {
		return err
	}

	// extract DID from repo commit
	sc := r.SignedCommit()
	did, err := syntax.ParseDID(sc.Did)
	if err != nil {
		return err
	}

	topDir := did.String()
	fmt.Printf("writing output to: %s\n", topDir)

	// first the commit object as a meta file
	commitPath := topDir + "/_commit"
	os.MkdirAll(filepath.Dir(commitPath), os.ModePerm)
	recJson, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(commitPath+".json", recJson, 0666); err != nil {
		return err
	}

	// then all the actual records
	err = r.ForEach(ctx, "", func(k string, v cid.Cid) error {
		_, rec, err := r.GetRecord(ctx, k)
		if err != nil {
			return err
		}

		recPath := topDir + "/" + k
		fmt.Printf("%s.json\n", recPath)
		os.MkdirAll(filepath.Dir(recPath), os.ModePerm)
		if err != nil {
			return err
		}
		recJson, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(recPath+".json", recJson, 0666); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
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
	xrpcc, err = keepAlive(ctx, xrpcc)
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

		xrpcc, err = keepAlive(ctx, xrpcc)
		if err != nil {
			return fmt.Errorf("failed to execute login: %v", err)
		}
		cursor := ""
		resp, err := bsky.FeedGetAuthorFeed(ctx, &xrpcc, raw, cursor, "posts_with_media", false, 2)
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
					getImages(embed.EmbedImages_View.Images, post.Post.Cid)
				}
				if embed.EmbedVideo_View != nil {

				}
				if embed.EmbedExternal_View != nil {

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

func getImages(images []*bsky.EmbedImages_ViewImage, ID string) error {
	for counter, image := range images {
		if err := os.MkdirAll("files", os.ModePerm); err != nil {
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

		filePath := filepath.Join("files", fmt.Sprintf("%s-%s-%03d.jpg", image.Alt, ID, counter))

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
