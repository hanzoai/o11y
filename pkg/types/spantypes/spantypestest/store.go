package spantypestest

import (
	"github.com/hanzoai/o11y/pkg/telemetrystore/datastoremock"
	"github.com/hanzoai/o11y/pkg/types/spantypes"
)

// TraceStoreTest pairs a TraceStore with the Datastore mock.
type TraceStoreTest struct {
	store spantypes.TraceStore
	mock  datastoremock.Conn
}

func New(store spantypes.TraceStore, mock datastoremock.Conn) *TraceStoreTest {
	return &TraceStoreTest{store: store, mock: mock}
}

// Store returns the TraceStore for calling methods under test.
func (t *TraceStoreTest) Store() spantypes.TraceStore { return t.store }

// Mock returns the Datastore mock for setting query expectations.
func (t *TraceStoreTest) Mock() datastoremock.Conn { return t.mock }
