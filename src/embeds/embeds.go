package embeds

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	atproto "github.com/bluesky-social/indigo/api/atproto"
	bsky "github.com/bluesky-social/indigo/api/bsky"
)

func EmbedResolve(post *bsky.FeedDefs_FeedViewPost, account string) {
	embed := post.Post.Embed
	if embed.EmbedImages_View != nil {
		getImages(embed.EmbedImages_View.Images, post.Post.Cid, account)
	}
	if embed.EmbedVideo_View != nil {
		fmt.Printf("Video Found")
		var buf bytes.Buffer

		// Marshal the Record value to CBOR
		err := post.Post.Record.Val.MarshalCBOR(&buf)
		if err != nil {
			fmt.Printf("Failed to marshal to CBOR: %v", err)
		}

		// Get the CBOR bytes from the buffer
		cborBytes := buf.Bytes()

		// Now unmarshal back to a FeedPost
		var feedPost bsky.FeedPost
		err = feedPost.UnmarshalCBOR(bytes.NewReader(cborBytes))
		if err != nil {
			fmt.Printf("Failed to unmarshal from CBOR: %v", err)
		}

	}
	if embed.EmbedExternal_View != nil {
		fmt.Printf("External Found")
	}
	if embed.EmbedRecordWithMedia_View != nil {
		if embed.EmbedRecordWithMedia_View.Media.EmbedImages_View != nil {
			getImages(embed.EmbedRecordWithMedia_View.Media.EmbedImages_View.Images, post.Post.Author.Handle, account)
		}
	}
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

func saveBlobs() {
	data, err := atproto.SyncGetBlob(context.Background(), client, blob.Ref.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch blob: %v", err)
	}

	if err := os.WriteFile(blobPath, data, 0666); err != nil {
		return err
	}
	fmt.Printf("%s\tdownloaded\n", blobPath)
}
