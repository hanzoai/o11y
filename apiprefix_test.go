package o11y_test

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every o11y route is /v1/o11y/…: the host is api.hanzo.ai, so a path never
// says api again. The one /api/ segment served is the Sentry SDK's envelope
// suffix under /v1/o11y/api/, which a stock SDK appends to its DSN.

func TestNoRouteIsServedOutsideV1(t *testing.T) {
	for route := range registered(t, mounted(t)) {
		_, path, _ := strings.Cut(route, " ")
		if !strings.HasPrefix(path, "/v1/") {
			t.Errorf("%s is served outside /v1/", route)
		}
	}
}

// versioned is the spelling every first-party route had before /v1/o11y.
var versioned = regexp.MustCompile(`/api/v[0-9]`)

// Matches preceded by one of these are not o11y routes: TypeScript module
// paths (types/api/v5/…), Go modules (etcd/api/v3, alertmanager/api/v2) and
// the Sentry DSN namespace (/v1/o11y/api/…).
var notARoute = []string{"types", "etcd", "alertmanager", "/v1/o11y"}

var skipDirs = map[string]bool{".git": true, "node_modules": true, "build": true, "dist": true}

func TestNoCallerSpellsAPI(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if path == "apiprefix_test.go" || !scanned(path) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		s := bufio.NewScanner(bytes.NewReader(b))
		s.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for n := 1; s.Scan(); n++ {
			line := s.Text()
			for _, loc := range versioned.FindAllStringIndex(line, -1) {
				if !excused(line[:loc[0]]) {
					t.Errorf("%s:%d calls a route under /api/: %s", path, n, strings.TrimSpace(line))
				}
			}
		}
		return s.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
}

func scanned(path string) bool {
	switch filepath.Base(path) {
	case "Dockerfile", "Dockerfile.site", "Makefile":
		return true
	}
	switch filepath.Ext(path) {
	case ".go", ".ts", ".tsx", ".js", ".mjs", ".py", ".sh", ".yml", ".yaml", ".json", ".md", ".mdc", ".conf", ".env":
		return true
	}
	return false
}

func excused(before string) bool {
	for _, p := range notARoute {
		if strings.HasSuffix(before, p) {
			return true
		}
	}
	return false
}
