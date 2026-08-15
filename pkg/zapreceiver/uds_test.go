package zapreceiver_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/hanzoai/o11y/pkg/zapreceiver"
)

// A path binds a socket, and the socket is on disk where the sender expects it.
//
// This is the property the change exists for, and it is worth a test rather
// than a reading: both emit sides already passed the address through untouched,
// so the receiver was the only half that could not represent a path — and
// nothing failed loudly when it could not, it simply bound a TCP port instead.
func TestListenOnAPathBindsASocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "spans.sock")

	r, err := zapreceiver.New(zapreceiver.Config{
		Listen:  sock,
		OnBatch: func(context.Context, *zapreceiver.SpanBatch) error { return nil },
	})
	if err != nil {
		t.Fatalf("New with a socket path: %v", err)
	}
	defer r.Stop()

	fi, err := os.Stat(sock)
	if err != nil {
		t.Fatalf("nothing bound at %s: %v", sock, err)
	}
	if fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("%s exists but is not a socket (mode %v)", sock, fi.Mode())
	}

	// It answers: a sender that dials the path gets a connection, which is the
	// half a stat cannot show.
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial %s: %v", sock, err)
	}
	_ = c.Close()
}

// A host:port still binds TCP, so the off-pod senders that must keep reaching
// this receiver — the otel agent on every node, the gateway, another cluster
// through the otlz door — are unaffected. The socket is ADDITIVE.
func TestListenOnAPortStillBindsTCP(t *testing.T) {
	r, err := zapreceiver.New(zapreceiver.Config{
		Listen:  "127.0.0.1:0",
		OnBatch: func(context.Context, *zapreceiver.SpanBatch) error { return nil },
	})
	if err != nil {
		t.Fatalf("New with host:port: %v", err)
	}
	r.Stop()
}

// A malformed TCP address is still refused at construction. The port parse did
// not go away with the narrowing — it now applies only where a port is meant.
func TestAMalformedPortIsStillRefused(t *testing.T) {
	_, err := zapreceiver.New(zapreceiver.Config{
		Listen:  "127.0.0.1:not-a-port",
		OnBatch: func(context.Context, *zapreceiver.SpanBatch) error { return nil },
	})
	if err == nil {
		t.Fatal("expected an invalid Listen to be refused")
	}
}
