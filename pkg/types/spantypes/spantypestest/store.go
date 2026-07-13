package spantypestest

import (
	dsmock "github.com/hanzoai/datastore-go-mock"
	"github.com/hanzoai/o11y/pkg/types/spantypes"
)

// TraceStoreTest pairs a TraceStore with the Datastore mock.
type TraceStoreTest struct {
	store spantypes.TraceStore
	mock  dsmock.ClickConnMockCommon
}

func New(store spantypes.TraceStore, mock dsmock.ClickConnMockCommon) *TraceStoreTest {
	return &TraceStoreTest{store: store, mock: mock}
}

// Store returns the TraceStore for calling methods under test.
func (t *TraceStoreTest) Store() spantypes.TraceStore { return t.store }

// Mock returns the Datastore mock for setting query expectations.
func (t *TraceStoreTest) Mock() dsmock.ClickConnMockCommon { return t.mock }
