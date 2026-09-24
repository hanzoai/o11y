package datastoretelemetrystore

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
)

func config(dsn string, token func(context.Context) (string, error)) telemetrystore.Config {
	return telemetrystore.Config{
		Provider:   "datastore",
		Connection: telemetrystore.ConnectionConfig{MaxOpenConns: 1, MaxIdleConns: 1, DialTimeout: time.Second},
		Datastore:  telemetrystore.DatastoreConfig{DSN: dsn, Token: token},
	}
}

func token(context.Context) (string, error) { return "iam.token.for-datastore", nil }

// The warehouse admits an IAM identity and nothing else, so a store with no
// identity to present is refused at construction rather than connected some
// other way.
func TestNoIdentityIsNoStore(t *testing.T) {
	_, err := New(context.Background(), instrumentationtest.New().ToProviderSettings(), config("tcp://127.0.0.1:9000", nil))
	if err == nil {
		t.Fatal("a store with no IAM identity was built")
	}
}

// A DSN says where the warehouse is. One that also says who is asking is the
// retired password path, and it is refused rather than half-honoured.
func TestADSNThatNamesACredentialIsRefused(t *testing.T) {
	for _, dsn := range []string{
		"tcp://default:hunter2@127.0.0.1:9000",
		"tcp://default@127.0.0.1:9000",
	} {
		if _, err := New(context.Background(), instrumentationtest.New().ToProviderSettings(), config(dsn, token)); err == nil {
			t.Errorf("DSN %q carrying a credential built a store", dsn)
		}
	}
}

// With an identity and a bare address the store is built, and the handshake of
// every connection it opens carries the identity's token in the password field
// under the JWT marker — never a stored user or password.
func TestTheStorePresentsTheIdentity(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	hello := make(chan []byte, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 4096)
		n, _ := c.Read(buf)
		hello <- buf[:n]
	}()

	s, err := New(context.Background(), instrumentationtest.New().ToProviderSettings(), config("tcp://"+ln.Addr().String(), token))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = s.Datastore().Ping(ctx) // the fake answers nothing; the handshake is the point

	select {
	case h := <-hello:
		if !bytes.Contains(h, []byte(" JWT AUTHENTICATION ")) || !bytes.Contains(h, []byte("iam.token.for-datastore")) {
			t.Fatalf("the handshake carried %q; want the JWT marker and the identity's token", h)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the store never dialed the warehouse")
	}
}
