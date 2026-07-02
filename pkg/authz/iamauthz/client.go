// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package iamauthz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// iamClient is a minimal HTTP+JSON client for the Hanzo IAM Casbin API.
//
// It speaks the canonical Hanzo IAM surface under /v1/iam/: batch-enforce for
// authorization decisions, and add-policy / remove-policy / get-policies for
// relationship-tuple storage. Zero gRPC, zero protobuf.
//
// A relationship tuple (User, Relation, Object) maps directly onto a Casbin
// request/policy triple (sub, obj, act) = (User, Object, Relation).
type iamClient struct {
	endpoint     string // base URL, e.g. https://iam.hanzo.ai (no trailing slash)
	enforcerID   string // IAM enforcer id in "owner/name" form
	clientID     string
	clientSecret string
	httpc        *http.Client
}

// newIAMClient builds the client from config plus secret credentials pulled from
// the environment (O11Y_IAM_CLIENT_ID / O11Y_IAM_CLIENT_SECRET) so that secrets
// never live in config files.
func newIAMClient(config authz.IAMConfig) *iamClient {
	endpoint := strings.TrimRight(config.URL, "/")
	if endpoint == "" {
		endpoint = "https://iam.hanzo.ai"
	}

	enforcerID := config.EnforcerID
	if enforcerID == "" {
		enforcerID = "hanzo/o11y"
	}

	return &iamClient{
		endpoint:     endpoint,
		enforcerID:   enforcerID,
		clientID:     os.Getenv("O11Y_IAM_CLIENT_ID"),
		clientSecret: os.Getenv("O11Y_IAM_CLIENT_SECRET"),
		httpc:        &http.Client{Timeout: 10 * time.Second},
	}
}

// authzRule mirrors Hanzo IAM's Casbin policy row (util.AuthzRule).
type authzRule struct {
	Ptype string `json:"ptype"`
	V0    string `json:"v0"`
	V1    string `json:"v1"`
	V2    string `json:"v2"`
	V3    string `json:"v3"`
	V4    string `json:"v4"`
	V5    string `json:"v5"`
}

// tupleKey converts a stored policy row back into an o11y tuple.
func (r authzRule) tupleKey() *authtypes.TupleKey {
	return &authtypes.TupleKey{User: r.V0, Object: r.V1, Relation: r.V2}
}

// ruleFromTuple maps an o11y tuple onto a Casbin "p" policy row.
func ruleFromTuple(t *authtypes.TupleKey) authzRule {
	return authzRule{Ptype: "p", V0: t.GetUser(), V1: t.GetObject(), V2: t.GetRelation()}
}

// request builds the Casbin request triple (sub, obj, act) for a tuple.
func requestFromTuple(t *authtypes.TupleKey) []string {
	return []string{t.GetUser(), t.GetObject(), t.GetRelation()}
}

// iamResponse is the Hanzo IAM response envelope.
type iamResponse struct {
	Status string          `json:"status"`
	Msg    string          `json:"msg"`
	Data   json.RawMessage `json:"data"`
}

// post issues an authenticated POST to {endpoint}/v1/iam/{action} with the given
// query and JSON body, and returns the decoded response envelope.
func (c *iamClient) post(ctx context.Context, action string, query url.Values, body any) (*iamResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, action, query, bytes.NewReader(payload))
}

// get issues an authenticated GET to {endpoint}/v1/iam/{action}.
func (c *iamClient) get(ctx context.Context, action string, query url.Values) (*iamResponse, error) {
	return c.do(ctx, http.MethodGet, action, query, nil)
}

func (c *iamClient) do(ctx context.Context, method, action string, query url.Values, body io.Reader) (*iamResponse, error) {
	u := fmt.Sprintf("%s/v1/iam/%s", c.endpoint, action)
	if encoded := query.Encode(); encoded != "" {
		u += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.clientID != "" || c.clientSecret != "" {
		req.SetBasicAuth(c.clientID, c.clientSecret)
	}

	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var envelope iamResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("iam: decode response from %s: %w", action, err)
	}
	if envelope.Status != "ok" {
		return nil, fmt.Errorf("iam: %s returned status %q: %s", action, envelope.Status, envelope.Msg)
	}

	return &envelope, nil
}

// batchEnforce evaluates many Casbin requests, returning one allow-decision per
// request in the same order. The enforcerId-scoped batch endpoint nests the
// per-request results one level deep: data = [[r0, r1, ...]].
func (c *iamClient) batchEnforce(ctx context.Context, requests [][]string) ([]bool, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	resp, err := c.post(ctx, "batch-enforce", url.Values{"enforcerId": {c.enforcerID}}, requests)
	if err != nil {
		return nil, err
	}

	var nested [][]bool
	if err := json.Unmarshal(resp.Data, &nested); err != nil {
		return nil, fmt.Errorf("iam: decode batch-enforce data: %w", err)
	}
	if len(nested) == 0 {
		return make([]bool, len(requests)), nil
	}

	return nested[0], nil
}

// addPolicy stores a relationship tuple as a Casbin "p" policy. Idempotent:
// IAM returns affected=false (not an error) when the policy already exists.
func (c *iamClient) addPolicy(ctx context.Context, t *authtypes.TupleKey) error {
	_, err := c.post(ctx, "add-policy", url.Values{"id": {c.enforcerID}}, ruleFromTuple(t))
	return err
}

// removePolicy deletes a relationship tuple. Idempotent: removing a missing
// policy is a no-op (affected=false).
func (c *iamClient) removePolicy(ctx context.Context, t *authtypes.TupleKey) error {
	_, err := c.post(ctx, "remove-policy", url.Values{"id": {c.enforcerID}}, ruleFromTuple(t))
	return err
}

// getPolicies returns every stored "p" policy for o11y's enforcer as tuples.
func (c *iamClient) getPolicies(ctx context.Context) ([]*authtypes.TupleKey, error) {
	resp, err := c.get(ctx, "get-policies", url.Values{"id": {c.enforcerID}})
	if err != nil {
		return nil, err
	}

	var rules []authzRule
	if err := json.Unmarshal(resp.Data, &rules); err != nil {
		return nil, fmt.Errorf("iam: decode get-policies data: %w", err)
	}

	tuples := make([]*authtypes.TupleKey, 0, len(rules))
	for _, rule := range rules {
		if rule.Ptype != "" && rule.Ptype != "p" {
			continue
		}
		tuples = append(tuples, rule.tupleKey())
	}
	return tuples, nil
}
