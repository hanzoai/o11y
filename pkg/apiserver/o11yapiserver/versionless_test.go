// Copyright 2025 Hanzo AI Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package o11yapiserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

// TestAddVersionlessAliases proves the generated version-less aliases resolve the
// three collision classes correctly: a resource in several O11y versions aliases to
// the HIGHEST (forward-only), a name owned by an llmobs route is left to llmobs, and
// the original versioned routes keep working.
func TestAddVersionlessAliases(t *testing.T) {
	h := func(id string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, id) }
	}
	r := mux.NewRouter()
	// O11y native (versioned).
	r.Handle("/api/v1/health", h("v1-health")).Methods("GET")
	r.Handle("/api/v1/services", h("v1-services")).Methods("GET")
	r.Handle("/api/v3/query_range", h("v3-qr")).Methods("POST")
	r.Handle("/api/v4/query_range", h("v4-qr")).Methods("POST")
	r.Handle("/api/v5/query_range", h("v5-qr")).Methods("POST")
	r.Handle("/api/v1/dashboards/{id}", h("v1-dash")).Methods("GET")
	r.Handle("/api/v3/dashboards/{id}", h("v3-dash")).Methods("GET")
	r.Handle("/api/v1/traces", h("v1-apmtraces")).Methods("GET") // name-clash with llmobs
	// Hanzo llmobs (non-versioned) — owns its names; registered before aliasing.
	r.Handle("/api/traces", h("llmobs-traces")).Methods("GET")

	if err := AddVersionlessAliases(r); err != nil {
		t.Fatalf("AddVersionlessAliases: %v", err)
	}

	serve := func(method, path string) string {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		if rec.Code != http.StatusOK {
			return fmt.Sprintf("HTTP %d", rec.Code)
		}
		return rec.Body.String()
	}
	for _, c := range []struct{ method, path, want string }{
		{"GET", "/api/health", "v1-health"},       // version-less → v1
		{"GET", "/api/services", "v1-services"},   // version-less → v1
		{"POST", "/api/query_range", "v5-qr"},     // multi-version → HIGHEST (v5)
		{"GET", "/api/dashboards/42", "v3-dash"},  // multi-version + path var → v3
		{"GET", "/api/traces", "llmobs-traces"},   // llmobs OWNS the name (not APM v1)
		{"POST", "/api/v5/query_range", "v5-qr"},  // original versioned still resolves
		{"GET", "/api/v1/traces", "v1-apmtraces"}, // APM traces reachable via its version
	} {
		if got := serve(c.method, c.path); got != c.want {
			t.Errorf("%s %s = %q, want %q", c.method, c.path, got, c.want)
		}
	}
}
