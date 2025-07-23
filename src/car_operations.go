package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	_ "github.com/bluesky-social/indigo/api/chat"
	_ "github.com/bluesky-social/indigo/api/ozone"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	auth "github.com/viniciusrf/bsky_monitor_go/src/auth"
)

func carDownload(raw string) error {
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
