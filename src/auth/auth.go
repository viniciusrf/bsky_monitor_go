package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	_ "github.com/bluesky-social/indigo/api/chat"
	_ "github.com/bluesky-social/indigo/api/ozone"
	"github.com/bluesky-social/indigo/atproto/identity"
	syntax "github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
)

var lastRefresh time.Time = time.Now()
var Ident *identity.Identity

func authLogin(ctx context.Context, xrpc *xrpc.Client) (*comatproto.ServerCreateSession_Output, error) {
	var authInfo comatproto.ServerCreateSession_Input
	authInfo.Identifier = os.Getenv("BSKY_USER")
	authInfo.Password = os.Getenv("BSKY_PASS")

	auth, err := comatproto.ServerCreateSession(ctx, xrpc, &authInfo)
	if err != nil {
		return nil, fmt.Errorf("Failed to get ServerCreateSession_Output: %v - authInfo = %v", err, xrpc)
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

func KeepAlive(ctx context.Context, xrpcc xrpc.Client) (xrpc.Client, error) {

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
		fmt.Printf("Refreshed token @ : %s ", time.Now().Format(time.RFC1123))
		xrpcc.Auth = &xrpc.AuthInfo{
			AccessJwt:  authData.AccessJwt,
			RefreshJwt: authData.RefreshJwt,
			Did:        authData.Did,
			Handle:     authData.Handle,
		}
	}

	return xrpcc, nil

}

func ParseUserHandle(raw string) (*identity.Identity, error) {
	ctx := context.Background()
	atid, err := syntax.ParseAtIdentifier(raw)
	if err != nil {
		return nil, fmt.Errorf("Failed to get ParseAtIdentifier: %v", err)
	}

	// first look up the DID and PDS for this repo
	dir := identity.DefaultDirectory()
	Ident, err = dir.Lookup(ctx, *atid)
	if err != nil {
		return nil, fmt.Errorf("Failed to get DefaultDirectory-Lookup: %v", err)
	}
	return Ident, nil
}
