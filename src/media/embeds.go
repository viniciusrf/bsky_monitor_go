package media

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	bsky "github.com/bluesky-social/indigo/api/bsky"
)

func EmbedResolve(post *bsky.FeedDefs_FeedViewPost, account string) {
	embed := post.Post.Embed
	if embed.EmbedImages_View != nil {
		getImages(post)
	}
	if embed.EmbedVideo_View != nil {
		getVideo(post)
	}
	if embed.EmbedExternal_View != nil {
		fmt.Printf("External Found")
	}
	if embed.EmbedRecordWithMedia_View != nil {
		if embed.EmbedRecordWithMedia_View.Media.EmbedImages_View != nil {
			getImages(post)
		}
	}
}

func getImages(post *bsky.FeedDefs_FeedViewPost) {
	ID := post.Post.Cid
	userHandle := post.Post.Author.Handle
	var images []*bsky.EmbedImages_ViewImage
	if post.Post.Embed.EmbedImages_View != nil {
		images = post.Post.Embed.EmbedImages_View.Images
	} else {
		images = post.Post.Embed.EmbedRecordWithMedia_View.Media.EmbedImages_View.Images
	}

	for counter, image := range images {
		folder := "files/" + userHandle
		if err := os.MkdirAll(folder, os.ModePerm); err != nil {
			fmt.Printf("failed to create folder: %v", err)
		}

		resp, err := http.Get(image.Fullsize)
		if err != nil {
			fmt.Printf("failed to download image: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("failed to download image: status code %d", resp.StatusCode)
		}

		filePath := filepath.Join(folder, fmt.Sprintf("%s-%s-%03d.jpg", strings.ReplaceAll(image.Alt, " ", "-"), ID, counter))

		file, err := os.Create(filePath)
		if err != nil {
			fmt.Printf("failed to create file: %v", err)
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			fmt.Printf("failed to save image: %v", err)
		}

		fmt.Printf("Image saved: %s\n", filePath)
	}
}

func getVideo(post *bsky.FeedDefs_FeedViewPost) {
	var buf bytes.Buffer
	author := post.Post.Author.Handle
	err := post.Post.Record.Val.MarshalCBOR(&buf)
	if err != nil {
		fmt.Printf("Failed to marshal to CBOR: %v", err)
	}

	cborBytes := buf.Bytes()

	var feedPost bsky.FeedPost
	err = feedPost.UnmarshalCBOR(bytes.NewReader(cborBytes))
	if err != nil {
		fmt.Printf("Failed to unmarshal from CBOR: %v", err)
	}

	saveBlobs(feedPost.Embed.EmbedVideo.Video, post.Post.Cid, author)
}
