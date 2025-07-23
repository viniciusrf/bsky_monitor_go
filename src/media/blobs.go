package media

import (
	"context"
	"fmt"
	"os"
	"strings"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"

	auth "github.com/viniciusrf/bsky_monitor_go/src/auth"
)

func BlobList(raw string) error {
	ctx := context.Background()
	ident, err := auth.ParseUserHandle(raw)
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

func BlobDownloadAll(raw string) error {
	ctx := context.Background()
	ident, err := auth.ParseUserHandle(raw)
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

func saveBlobs(blobRef *util.LexBlob, name, author string) {
	xrpcc := xrpc.Client{
		Host: auth.Ident.PDSEndpoint(),
	}
	if xrpcc.Host == "" {
		fmt.Printf("no PDS endpoint for identity")
	}

	topDir := "files/" + author + "/_blob"
	fmt.Printf("writing blobs to: %s\n", topDir)
	os.MkdirAll(topDir, os.ModePerm)

	data, err := comatproto.SyncGetBlob(context.Background(), &xrpcc, blobRef.Ref.String(), auth.Ident.DID.String())
	if err != nil {
		fmt.Printf("failed to fetch blob: %v", err)
	}
	extension := "." + getExtensionFromMIME(blobRef.MimeType)
	blobPath := topDir + "/" + name + extension
	if err := os.WriteFile(blobPath, data, 0666); err != nil {
		fmt.Printf("failed to save blob: %v", err)
	}
	fmt.Printf("%s\tdownloaded\n", blobPath)
}

func getExtensionFromMIME(mime string) string {
	parts := strings.Split(mime, "/")
	subtype := ""
	if len(parts) > 1 {
		subtype = parts[1]
		fmt.Println(subtype)
	} else {
		fmt.Println("No subtype found")
	}
	return subtype
}
