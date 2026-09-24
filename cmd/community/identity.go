package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// warehouse is the audience a warehouse token is minted for (RFC 8707). The
// warehouse admits a token for this audience and nothing else.
const warehouse = "datastore"

// renewLead is how long before expiry a token is replaced: it is presented at a
// handshake the connection then outlives, so one about to lapse is never handed
// out.
const renewLead = 2 * time.Minute

// identity is this server's own IAM identity for the warehouse: the
// client_credentials grant for IAM_CLIENT_ID / IAM_CLIENT_SECRET at IAM_URL,
// scoped to the datastore audience, cached and renewed ahead of expiry. The
// server holds no warehouse credential of any other kind, so without an identity
// it does not start.
func identity() (func(context.Context) (string, error), error) {
	id := strings.TrimSpace(os.Getenv("IAM_CLIENT_ID"))
	secret := os.Getenv("IAM_CLIENT_SECRET")
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_URL")), "/")
	if id == "" || secret == "" || base == "" {
		return nil, fmt.Errorf("the warehouse admits an IAM identity and nothing else: set IAM_URL, IAM_CLIENT_ID and IAM_CLIENT_SECRET")
	}
	cc := &clientcredentials.Config{
		ClientID:       id,
		ClientSecret:   secret,
		TokenURL:       base + "/v1/iam/oauth/token",
		AuthStyle:      oauth2.AuthStyleInParams,
		EndpointParams: url.Values{"resource": {warehouse}},
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: 30 * time.Second})
	src := oauth2.ReuseTokenSourceWithExpiry(nil, mint{cc: cc, ctx: ctx}, renewLead)
	return func(context.Context) (string, error) {
		t, err := src.Token()
		if err != nil {
			return "", fmt.Errorf("warehouse identity: %w", err)
		}
		return t.AccessToken, nil
	}, nil
}

// mint asks IAM for a fresh token every call; the one cache is identity's.
type mint struct {
	cc  *clientcredentials.Config
	ctx context.Context
}

func (m mint) Token() (*oauth2.Token, error) { return m.cc.Token(m.ctx) }
