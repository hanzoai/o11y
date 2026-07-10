package implerrortracking

import (
	"testing"

	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func frame(fn, module, file string, inApp bool) errortrackingtypes.Frame {
	return errortrackingtypes.Frame{Function: fn, Module: module, Filename: file, InApp: inApp}
}

// Same crash site (type + top frame) groups even when the message varies.
func TestFingerprint_GroupsBySameCrashFrame(t *testing.T) {
	a := &errortrackingtypes.Occurrence{Type: "ValueError", Value: "id 12 bad", Frames: []errortrackingtypes.Frame{frame("handle", "app.svc", "svc.py", true)}}
	b := &errortrackingtypes.Occurrence{Type: "ValueError", Value: "id 999 bad", Frames: []errortrackingtypes.Frame{frame("handle", "app.svc", "svc.py", true)}}
	assert.Equal(t, computeFingerprint(a, nil), computeFingerprint(b, nil), "same type+frame must group regardless of message")
}

func TestFingerprint_DistinctForDifferentTypes(t *testing.T) {
	a := &errortrackingtypes.Occurrence{Type: "ValueError", Frames: []errortrackingtypes.Frame{frame("handle", "app.svc", "svc.py", true)}}
	b := &errortrackingtypes.Occurrence{Type: "KeyError", Frames: []errortrackingtypes.Frame{frame("handle", "app.svc", "svc.py", true)}}
	assert.NotEqual(t, computeFingerprint(a, nil), computeFingerprint(b, nil))
}

// The crash frame is the innermost in-app frame (Sentry orders caller→callee).
func TestFingerprint_PicksInnermostInAppFrame(t *testing.T) {
	frames := []errortrackingtypes.Frame{
		frame("main", "app", "main.go", true),
		frame("libcall", "vendor.lib", "lib.go", false),
		frame("crashHere", "app.worker", "worker.go", true),
		frame("runtimePanic", "runtime", "panic.go", false),
	}
	got := pickCrashFrame(frames)
	require.NotNil(t, got)
	assert.Equal(t, "crashHere", got.Function, "innermost in-app frame is the crash site, not the runtime frame")
}

// Message-only errors group by normalized message (numbers/uuids masked).
func TestFingerprint_MessageFallbackMasksVariadic(t *testing.T) {
	a := &errortrackingtypes.Occurrence{Type: "Message", Value: "user 123 not found in shard 7"}
	b := &errortrackingtypes.Occurrence{Type: "Message", Value: "user 456 not found in shard 9"}
	assert.Equal(t, computeFingerprint(a, nil), computeFingerprint(b, nil))

	c := &errortrackingtypes.Occurrence{Type: "Message", Value: "totally different failure"}
	assert.NotEqual(t, computeFingerprint(a, nil), computeFingerprint(c, nil))
}

func TestFingerprint_CustomHonored(t *testing.T) {
	a := &errortrackingtypes.Occurrence{Type: "ValueError", Value: "x"}
	b := &errortrackingtypes.Occurrence{Type: "KeyError", Value: "y"}
	// Same explicit fingerprint => same group despite different types.
	assert.Equal(t, computeFingerprint(a, []string{"my-group"}), computeFingerprint(b, []string{"my-group"}))
	assert.NotEqual(t, computeFingerprint(a, []string{"g1"}), computeFingerprint(a, []string{"g2"}))
}

// "{{ default }}" expands to the computed default, so ["{{ default }}", "tenant"]
// subdivides the default group by tenant.
func TestFingerprint_DefaultTokenExpands(t *testing.T) {
	occ := &errortrackingtypes.Occurrence{Type: "ValueError", Frames: []errortrackingtypes.Frame{frame("h", "m", "f.go", true)}}
	base := computeFingerprint(occ, nil)
	withToken := computeFingerprint(occ, []string{defaultFingerprintToken})
	assert.Equal(t, base, withToken, "bare {{ default }} equals the default fingerprint")

	subdivided := computeFingerprint(occ, []string{defaultFingerprintToken, "tenant-a"})
	assert.NotEqual(t, base, subdivided, "adding a component must change the group")
}

func TestNormalizeMessage_Masks(t *testing.T) {
	assert.Equal(t, normalizeMessage("id 42 at 0xdeadbeef"), normalizeMessage("id 7 at 0xcafef00d"))
	assert.Equal(t,
		normalizeMessage("row 550e8400-e29b-41d4-a716-446655440000 gone"),
		normalizeMessage("row 550e8400-e29b-41d4-a716-000000000000 gone"),
	)
}

func TestFingerprint_IsHex(t *testing.T) {
	fp := computeFingerprint(&errortrackingtypes.Occurrence{Type: "E"}, nil)
	assert.Len(t, fp, 64)
}
