// Copyright (C) 2025-2026, Hanzo Industries Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package iamauthz

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// testClient returns an iamClient pointed at a mock IAM server.
func testClient(url string) *iamClient {
	return &iamClient{endpoint: url, enforcerID: "hanzo/o11y", httpc: http.DefaultClient}
}

func writeOK(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": data}); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

// TestBatchCheckMapsTuplesToCasbin asserts the decision path: each tuple is sent
// to /v1/iam/batch-enforce as a Casbin (sub,obj,act) = (User,Object,Relation)
// triple, and the enforcerId-scoped nested result ([[...]]) is decoded back onto
// each tuple id.
func TestBatchCheckMapsTuplesToCasbin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/iam/batch-enforce" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("enforcerId"); got != "hanzo/o11y" {
			t.Errorf("enforcerId = %q, want hanzo/o11y", got)
		}

		body, _ := io.ReadAll(r.Body)
		var requests [][]string
		if err := json.Unmarshal(body, &requests); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		want := [][]string{{"user:u", "dashboard:d", "read"}}
		if len(requests) != 1 || requests[0][0] != want[0][0] || requests[0][1] != want[0][1] || requests[0][2] != want[0][2] {
			t.Errorf("casbin request = %v, want %v", requests, want)
		}

		// enforcerId-scoped batch nests results one level deep.
		writeOK(t, w, [][]bool{{true}})
	}))
	defer srv.Close()

	p := &provider{iam: testClient(srv.URL)}
	resp, err := p.BatchCheck(context.Background(), map[string]*authtypes.TupleKey{
		"txn-1": {User: "user:u", Relation: "read", Object: "dashboard:d"},
	})
	if err != nil {
		t.Fatalf("BatchCheck: %v", err)
	}
	if !resp["txn-1"].Authorized {
		t.Errorf("txn-1 Authorized = false, want true")
	}
}

// TestBatchCheckFailsClosed asserts a denied decision maps to Authorized=false
// and a short result slice does not panic (missing → deny).
func TestBatchCheckFailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeOK(t, w, [][]bool{{false}}) // one result for two requests
	}))
	defer srv.Close()

	p := &provider{iam: testClient(srv.URL)}
	resp, err := p.BatchCheck(context.Background(), map[string]*authtypes.TupleKey{
		"a": {User: "user:u", Relation: "read", Object: "dashboard:1"},
		"b": {User: "user:u", Relation: "read", Object: "dashboard:2"},
	})
	if err != nil {
		t.Fatalf("BatchCheck: %v", err)
	}
	for id, r := range resp {
		if r.Authorized {
			t.Errorf("%s Authorized = true, want false (fail closed)", id)
		}
	}
}

// TestWritePostsCasbinPolicy asserts Write maps an addition tuple onto an
// add-policy AuthzRule {Ptype:"p", V0:User, V1:Object, V2:Relation}.
func TestWritePostsCasbinPolicy(t *testing.T) {
	var gotRule authzRule
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if got := r.URL.Query().Get("id"); got != "hanzo/o11y" {
			t.Errorf("id = %q, want hanzo/o11y", got)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotRule); err != nil {
			t.Fatalf("decode rule: %v", err)
		}
		writeOK(t, w, true)
	}))
	defer srv.Close()

	p := &provider{iam: testClient(srv.URL)}
	err := p.Write(context.Background(), []*authtypes.TupleKey{
		{User: "user:u", Relation: "assignee", Object: "role:admin"},
	}, nil)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if gotPath != "/v1/iam/add-policy" {
		t.Errorf("path = %q, want /v1/iam/add-policy", gotPath)
	}
	want := authzRule{Ptype: "p", V0: "user:u", V1: "role:admin", V2: "assignee"}
	if gotRule != want {
		t.Errorf("rule = %+v, want %+v", gotRule, want)
	}
}

// TestReadTuplesDecodesAndFilters asserts get-policies rows decode to tuples and
// the filter narrows by non-empty fields.
func TestReadTuplesDecodesAndFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/iam/get-policies" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		writeOK(t, w, []authzRule{
			{Ptype: "p", V0: "user:a", V1: "dashboard:1", V2: "read"},
			{Ptype: "p", V0: "user:b", V1: "dashboard:2", V2: "read"},
		})
	}))
	defer srv.Close()

	p := &provider{iam: testClient(srv.URL)}
	tuples, err := p.ReadTuples(context.Background(), &authtypes.ReadRequestTupleKey{User: "user:a"})
	if err != nil {
		t.Fatalf("ReadTuples: %v", err)
	}
	if len(tuples) != 1 || tuples[0].Object != "dashboard:1" {
		t.Fatalf("filtered tuples = %+v, want single dashboard:1", tuples)
	}
}
