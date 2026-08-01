package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// DUAL DISPATCH. These registrations stay: the standalone server reaches them
// directly, and the composed binary reaches the SAME handlers through the typed
// autocomplete ops in the repo root's querycore.go, which relay here. Deleting
// either half drops one of the two deployments.

// mountAutocomplete registers attribute autocomplete on the shared /v1/o11y
// subrouter owned by RegisterQueryRangeV3Routes. 4 routes.
//
// Both spellings live here: the three cache-controlled GET /autocomplete/*
// endpoints and the newer POST /auto_complete/attribute_values, which takes
// filters. Eventually all autocomplete APIs should migrate to the new endpoint
// with appropriate filters, deprecating the older ones.
func (aH *APIHandler) mountAutocomplete(subRouter *mux.Router, am *middleware.AuthZ) {
	subRouter.HandleFunc("/autocomplete/aggregate_attributes", am.ViewAccess(
		withCacheControl(AutoCompleteCacheControlAge, aH.autocompleteAggregateAttributes))).Methods(http.MethodGet)
	subRouter.HandleFunc("/autocomplete/attribute_keys", am.ViewAccess(
		withCacheControl(AutoCompleteCacheControlAge, aH.autoCompleteAttributeKeys))).Methods(http.MethodGet)
	subRouter.HandleFunc("/autocomplete/attribute_values", am.ViewAccess(
		withCacheControl(AutoCompleteCacheControlAge, aH.autoCompleteAttributeValues))).Methods(http.MethodGet)

	subRouter.HandleFunc("/auto_complete/attribute_values", am.ViewAccess(aH.autoCompleteAttributeValuesPost)).Methods(http.MethodPost)
}
