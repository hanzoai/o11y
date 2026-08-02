package alertmanagertypes

// A MAINTENANCE WINDOW WITH NO SCHEDULE MUST NOT TAKE THE PROCESS DOWN.
//
// Schedule is a POINTER, and three predicates in the marshalling path
// dereference it without asking whether it is there: IsActive reads
// Schedule.StartTime, IsUpcoming reads Schedule.EndTime, IsRecurring reads
// Schedule.Recurrence. MarshalJSON has a VALUE receiver and calls all three, so
// marshalling a PlannedMaintenance whose Schedule is nil is a nil dereference
// inside encoding/json — a panic, not an error.
//
// Validate() guards the INPUT path (it refuses a payload with no schedule) and
// the store column is notnull, so a row read back always has one. Neither
// guards the OUTPUT path, and the output path is now reachable: the downtime
// ops answer with O11yDowntimeScheduleOut{Data PlannedMaintenance} — a VALUE
// with `json:"data,omitempty"`, and omitempty does not omit a struct. Any
// runtime answer that carries no "data" key therefore decodes to the zero
// PlannedMaintenance, whose Schedule is nil, and marshalling it back to the
// caller panics.
//
// Those routes answered 404 until the relay collapse made them reachable, which
// is why this had never fired. A panic in a handler of the unified binary is not
// one endpoint failing — cloud serves iam, commerce, ai and the gateway from the
// same process.
//
// The honest semantics: a window with no schedule is not active, not upcoming
// and not recurring. It marshals as expired/fixed rather than killing the host.

import (
	"encoding/json"
	"testing"
	"time"
)

func TestZeroPlannedMaintenanceMarshalsWithoutPanicking(t *testing.T) {
	var m PlannedMaintenance // Schedule is nil

	got, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var back map[string]any
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatalf("Unmarshal: %v (body %s)", err, got)
	}
	if back["status"] != MaintenanceStatusExpired.StringValue() {
		t.Errorf("status = %v, want %q — a window with no schedule is not active",
			back["status"], MaintenanceStatusExpired)
	}
	if back["kind"] != MaintenanceKindFixed.StringValue() {
		t.Errorf("kind = %v, want %q — a window with no schedule is not recurring",
			back["kind"], MaintenanceKindFixed)
	}
}

// The predicates are the actual seam, so they are pinned directly: a caller that
// reaches them outside the marshaller gets the same answer instead of a panic.
func TestNilSchedulePredicatesAreFalse(t *testing.T) {
	m := &PlannedMaintenance{}
	if m.IsActive(time.Now()) {
		t.Error("IsActive = true for a window with no schedule")
	}
	if m.IsUpcoming() {
		t.Error("IsUpcoming = true for a window with no schedule")
	}
	if m.IsRecurring() {
		t.Error("IsRecurring = true for a window with no schedule")
	}
}

// The shape that carries a nil Schedule in production: a 2xx with no data key.
func TestMaintenanceDecodedFromAnEmptyAnswerMarshalsBack(t *testing.T) {
	var out struct {
		Data PlannedMaintenance `json:"data,omitempty"`
	}
	if err := json.Unmarshal([]byte(`{"status":"success"}`), &out); err != nil {
		t.Fatalf("decode the runtime's answer: %v", err)
	}
	if out.Data.Schedule != nil {
		t.Fatal("precondition: Schedule should be nil after decoding an answer with no data")
	}
	if _, err := json.Marshal(out); err != nil {
		t.Fatalf("re-marshalling the op's Out: %v", err)
	}
}
