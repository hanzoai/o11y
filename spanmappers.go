package o11y

// The SPAN-MAPPER face — the ingest-time rules that move or copy span
// attributes into resource attributes before a span is stored — as TYPED ops.
//
// These were eight of the routes that reached traffic only through the
// delegation wildcard, and a route behind a wildcard is in no document: no SDK
// method, no CLI command, no agent tool, no reference page. Span mappers are the
// one surface a customer must edit to fix their own telemetry shape, and until
// now they could only do it from a hand-written HTTP call. Typing them is what
// puts the eight operations in the document and therefore in every projection
// built from it.
//
// A GROUP AND ITS MAPPERS. A group carries the condition that decides WHICH
// spans it applies to; the mappers under it carry the moves. That nesting is the
// path shape (/span_mapper_groups/:groupId/span_mappers/:mapperId) and it is why
// the mapper ops take two ids.
//
// THE WIRE DOES NOT MOVE: each op hands the call to the SAME runtime handler the
// wildcard delegates to (relay.go), so the ViewAccess read gate and the
// AdminAccess write gate the runtime has always run still run, in the order they
// always did. The four writes answer 204 with no body, and the ops declare that
// status rather than inventing a payload.

//go:generate go run github.com/zap-proto/zip/cmd/zipdoc

import (
	"context"
	"net/http"

	"github.com/hanzoai/o11y/pkg/types/spantypes"
	"github.com/zap-proto/zip"
)

// mountSpanMappers registers the eight typed span-mapper ops.
func mountSpanMappers(app *zip.App) {
	// The group is o11yRoot, not o11yRoot+"/span_mapper_groups": a zip group
	// with the collection as its prefix would register the collection routes at
	// the empty sub-path, which lands them on a TRAILING SLASH the runtime does
	// not serve. The collection is a path, not a prefix.
	g := app.Group(o11yRoot)

	zip.Get(g, "/span_mapper_groups", spanMapperGroups, op("ListSpanMapperGroups"))
	zip.Post(g, "/span_mapper_groups", spanMapperGroupCreate, op("CreateSpanMapperGroup"), zip.WithStatus(http.StatusCreated))
	zip.Patch(g, "/span_mapper_groups/:groupId", spanMapperGroupUpdate, op("UpdateSpanMapperGroup"), zip.WithStatus(http.StatusNoContent))
	zip.Delete(g, "/span_mapper_groups/:groupId", spanMapperGroupDelete, op("DeleteSpanMapperGroup"), zip.WithStatus(http.StatusNoContent))

	zip.Get(g, "/span_mapper_groups/:groupId/span_mappers", spanMappers, op("ListSpanMappers"))
	zip.Post(g, "/span_mapper_groups/:groupId/span_mappers", spanMapperCreate, op("CreateSpanMapper"), zip.WithStatus(http.StatusCreated))
	zip.Patch(g, "/span_mapper_groups/:groupId/span_mappers/:mapperId", spanMapperUpdate, op("UpdateSpanMapper"), zip.WithStatus(http.StatusNoContent))
	zip.Delete(g, "/span_mapper_groups/:groupId/span_mappers/:mapperId", spanMapperDelete, op("DeleteSpanMapper"), zip.WithStatus(http.StatusNoContent))
}

// ── groups ────────────────────────────────────────────────────────────────────

// spanMapperGroups lists the caller's org's mapping groups, optionally only the
// enabled ones.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func spanMapperGroups(ctx context.Context, in *O11ySpanMapperGroupsIn) (*O11ySpanMapperGroupsOut, error) {
	out := new(O11ySpanMapperGroupsOut)
	return out, relay(ctx, http.MethodGet, o11yRoot+"/span_mapper_groups", query("enabled", in.Enabled), nil, out)
}

// spanMapperGroupCreate creates a mapping group: the name it is known by, the
// span and resource attributes whose presence selects a span into it, and
// whether it is on.
//
// Callers need the admin role; the runtime's own gate enforces it.
func spanMapperGroupCreate(ctx context.Context, in *spantypes.PostableSpanMapperGroup) (*O11ySpanMapperGroupOut, error) {
	out := new(O11ySpanMapperGroupOut)
	return out, relay(ctx, http.MethodPost, o11yRoot+"/span_mapper_groups", nil, in, out)
}

// spanMapperGroupUpdate changes a group's name, condition or enabled state.
// Every field is optional and only the ones sent are applied.
//
// Callers need the admin role; the runtime's own gate enforces it.
func spanMapperGroupUpdate(ctx context.Context, in *O11ySpanMapperGroupUpdateIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPatch, groupPath(in.GroupID), nil, in.UpdatableSpanMapperGroup, nil)
}

// spanMapperGroupDelete deletes a mapping group and every mapper under it.
//
// Callers need the admin role; the runtime's own gate enforces it.
func spanMapperGroupDelete(ctx context.Context, in *O11ySpanMapperGroupRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, groupPath(in.GroupID), nil, nil, nil)
}

// ── mappers ───────────────────────────────────────────────────────────────────

// spanMappers lists the mappers belonging to one group, in the order they are
// applied.
//
// Callers need the viewer role; the runtime's own gate enforces it.
func spanMappers(ctx context.Context, in *O11ySpanMapperGroupRef) (*O11ySpanMappersOut, error) {
	out := new(O11ySpanMappersOut)
	return out, relay(ctx, http.MethodGet, groupPath(in.GroupID)+"/span_mappers", nil, nil, out)
}

// spanMapperCreate adds a mapper to a group: which field context it reads, the
// move or copy it performs, and whether it is on.
//
// Callers need the admin role; the runtime's own gate enforces it.
func spanMapperCreate(ctx context.Context, in *O11ySpanMapperCreateIn) (*O11ySpanMapperOut, error) {
	out := new(O11ySpanMapperOut)
	return out, relay(ctx, http.MethodPost, groupPath(in.GroupID)+"/span_mappers", nil, in.PostableSpanMapper, out)
}

// spanMapperUpdate changes a mapper's field context, config or enabled state.
// Every field is optional and only the ones sent are applied.
//
// Callers need the admin role; the runtime's own gate enforces it.
func spanMapperUpdate(ctx context.Context, in *O11ySpanMapperUpdateIn) (*struct{}, error) {
	return nil, relay(ctx, http.MethodPatch, mapperPath(in.GroupID, in.MapperID), nil, in.UpdatableSpanMapper, nil)
}

// spanMapperDelete deletes one mapper from a group.
//
// Callers need the admin role; the runtime's own gate enforces it.
func spanMapperDelete(ctx context.Context, in *O11ySpanMapperRef) (*struct{}, error) {
	return nil, relay(ctx, http.MethodDelete, mapperPath(in.GroupID, in.MapperID), nil, nil, nil)
}

// groupPath and mapperPath are the one place these ids become a path. Each id
// goes on VERBATIM, as the segment the router matched.
func groupPath(groupID string) string { return o11yRoot + "/span_mapper_groups/" + groupID }

func mapperPath(groupID, mapperID string) string {
	return groupPath(groupID) + "/span_mappers/" + mapperID
}

// ── inputs ────────────────────────────────────────────────────────────────────

// O11ySpanMapperGroupsIn narrows the group listing.
type O11ySpanMapperGroupsIn struct {
	// Enabled lists only the groups that are on. False lists all of them —
	// which is what the runtime has always read an absent parameter as.
	Enabled bool `json:"-" url:"enabled"`
}

// O11ySpanMapperGroupRef names one mapping group.
type O11ySpanMapperGroupRef struct {
	// GroupID is the group's id. Required.
	GroupID string `json:"-" url:"groupId" validate:"required"`
}

// O11ySpanMapperGroupUpdateIn names the group and carries the change.
type O11ySpanMapperGroupUpdateIn struct {
	// GroupID is the group to update. Required.
	GroupID string `json:"-" url:"groupId" validate:"required"`

	spantypes.UpdatableSpanMapperGroup
}

// O11ySpanMapperRef names one mapper inside its group.
type O11ySpanMapperRef struct {
	// GroupID is the group the mapper belongs to. Required.
	GroupID string `json:"-" url:"groupId" validate:"required"`
	// MapperID is the mapper's id. Required.
	MapperID string `json:"-" url:"mapperId" validate:"required"`
}

// O11ySpanMapperCreateIn names the group and describes the mapper to add.
type O11ySpanMapperCreateIn struct {
	// GroupID is the group to add to. Required.
	GroupID string `json:"-" url:"groupId" validate:"required"`

	spantypes.PostableSpanMapper
}

// O11ySpanMapperUpdateIn names the mapper and carries the change.
type O11ySpanMapperUpdateIn struct {
	// GroupID is the group the mapper belongs to. Required.
	GroupID string `json:"-" url:"groupId" validate:"required"`
	// MapperID is the mapper to update. Required.
	MapperID string `json:"-" url:"mapperId" validate:"required"`

	spantypes.UpdatableSpanMapper
}

// ── answers ───────────────────────────────────────────────────────────────────

// O11ySpanMapperGroupsOut is the org's mapping groups.
type O11ySpanMapperGroupsOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the groups.
	Data spantypes.GettableSpanMapperGroups `json:"data"`
}

// O11ySpanMapperGroupOut is one mapping group.
type O11ySpanMapperGroupOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the group.
	Data spantypes.GettableSpanMapperGroup `json:"data"`
}

// O11ySpanMappersOut is one group's mappers.
type O11ySpanMappersOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data holds the mappers.
	Data spantypes.GettableSpanMappers `json:"data"`
}

// O11ySpanMapperOut is one mapper.
type O11ySpanMapperOut struct {
	// Status is "success".
	Status string `json:"status"`
	// Data is the mapper.
	Data spantypes.GettableSpanMapper `json:"data"`
}
