package o11y_test

import (
	"strings"
	"testing"

	"github.com/hanzoai/o11y"
	"github.com/zap-proto/zip"
)

// Composing the published table under a HOST prefix must not rename an
// operation whose id this module WROTE DOWN.
func TestProbe_PublishedIDsSurviveAHostPrefix(t *testing.T) {
	table := zip.New(zip.Config{AppName: "o11y", DisableStartupMessage: true})
	if err := o11y.Mount(table); err != nil {
		t.Fatalf("mount: %v", err)
	}
	var declared, derived []string
	for _, op := range table.Registry() {
		if strings.Contains(op.OperationID, ".") {
			derived = append(derived, op.OperationID) // shape-derived: v1.o11y.<method+path>
			continue
		}
		declared = append(declared, op.OperationID)
	}

	host := zip.New(zip.Config{AppName: "cloud", DisableStartupMessage: true})
	host.Group("/embedded").Use(table)
	composed := map[string]bool{}
	for _, op := range host.Registry() {
		composed[op.OperationID] = true
	}
	t.Logf("ops=%d declared=%d shape-derived=%d", len(table.Registry()), len(declared), len(derived))

	var lost []string
	for _, id := range declared {
		if !composed[id] {
			lost = append(lost, id)
		}
	}
	if len(lost) != 0 {
		t.Fatalf("%d DECLARED ids were renamed by composition, e.g. %v", len(lost), lost[:min(5, len(lost))])
	}
	t.Logf("all %d declared ids survived a host prefix verbatim", len(declared))
	t.Logf("%d ops declare no id and take the shape-derived one, which composition qualifies "+
		"by design — that is o11y's declaration gap, not zip's", len(derived))
}
