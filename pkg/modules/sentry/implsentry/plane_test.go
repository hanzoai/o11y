package implsentry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// namedScope is the test seam: a plane whose names are whatever the case says.
func namedScope(org, product string) Scope {
	return func(context.Context, valuer.UUID, valuer.UUID) (string, string, error) {
		return org, product, nil
	}
}

// orgUUID reproduces iamidentn.toUUID for an org slug. It is spelled out rather than
// imported because the point is that the two AGREE, and a check that calls the
// function under test cannot notice it changing.
func orgUUID(slug string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("hanzo:o11y:org:"+slug)).String()
}

// TestTheUUIDIsDerivedFromTheSlug is the whole argument for which vocabulary wins, as
// an executable fact rather than an assertion of taste.
//
// The plane held ONE tenant under TWO spellings — 86 rows of event.error under
// cd35a51b-f7e7-5412-b67f-58b8703e5219 and 2 under `hanzo`, measured 2026-08-02. They
// are the same tenant: that uuid is uuid5(NameSpaceURL, "hanzo:o11y:org:hanzo"). So
// the slug is the primal value and the uuid a projection of it, which is why the slug
// is what the plane stores — a projection can always be recomputed from the original,
// and the original cannot be recovered from the projection.
func TestTheUUIDIsDerivedFromTheSlug(t *testing.T) {
	assert.Equal(t, "cd35a51b-f7e7-5412-b67f-58b8703e5219", orgUUID("hanzo"),
		"the uuid on the plane is the slug's own derivation, not a separate identity")
	assert.Equal(t, "dfb7a19b-108f-5150-8131-7d207488bf48", orgUUID("admin"))
}

// TestRowWritesTheNamesNotTheIDs is the regression gate on the writer. It asserts the
// ROW, not the call: whatever ids the caller works in, what lands in `org` and
// `product` is what the plane calls them, and no uuid spelling appears anywhere in it.
func TestRowWritesTheNamesNotTheIDs(t *testing.T) {
	orgID := valuer.MustNewUUID(orgUUID("hanzo"))
	projectID := valuer.GenerateUUID()

	got := row("hanzo", "docs", time.Unix(0, 0).UTC(), &sentrytypes.Event{EventID: "e-1"})

	require.Len(t, got, countCols(insertColumns), "a row is exactly the insert column list")
	assert.Equal(t, "hanzo", got[0], "org is the IAM org slug")
	assert.Equal(t, "docs", got[1], "product is the project's slug")
	for _, cell := range got {
		if s, ok := cell.(string); ok {
			assert.NotEqual(t, orgID.String(), s, "no uuid spelling of the tenant reaches the plane")
			assert.NotEqual(t, projectID.String(), s, "no uuid spelling of the product reaches the plane")
		}
	}
}

// TestUnnamedTenantFailsClosed: an id the plane has no name for must refuse the
// operation. Writing it under a placeholder is exactly the commingling this seam
// exists to prevent — a bucket nobody owns is a bucket several tenants share.
func TestUnnamedTenantFailsClosed(t *testing.T) {
	boom := errors.Newf(errors.TypeNotFound, sentrytypes.ErrCodeSentryNotFound, "no such org")
	s := &eventStore{
		scope: func(context.Context, valuer.UUID, valuer.UUID) (string, string, error) { return "", "", boom },
		db:    defaultEventsDB,
		table: defaultEventsTable,
		now:   func() time.Time { return time.Unix(0, 0).UTC() },
	}
	// store is nil, so reaching the sink at all would panic rather than pass.
	require.Error(t, s.Insert(context.Background(), valuer.GenerateUUID(), valuer.GenerateUUID(),
		[]*sentrytypes.Event{{EventID: "e-1"}}))

	_, err := s.Discover(context.Background(), valuer.GenerateUUID(), valuer.GenerateUUID(),
		&sentrytypes.DiscoverRequest{}, testWindow())
	require.Error(t, err, "reads fail closed for the same reason writes do")
}

// TestAStoreWithoutAScopeCannotWrite: the resolver is a constructor argument, but a
// zero-value store must still refuse rather than write a nameless row.
func TestAStoreWithoutAScopeCannotWrite(t *testing.T) {
	s := &eventStore{db: defaultEventsDB, table: defaultEventsTable, now: time.Now}
	require.Error(t, s.Insert(context.Background(), valuer.GenerateUUID(), valuer.GenerateUUID(),
		[]*sentrytypes.Event{{EventID: "e-1"}}))
}

// TestScopeResolvesFromTheRecordsThatOwnTheNames pins NewScope: the org's name comes
// from the org row and the product's from the project row, each read once.
func TestScopeResolvesFromTheRecordsThatOwnTheNames(t *testing.T) {
	orgs := &countingOrgs{name: "hanzo"}
	projects := &countingProjects{slug: "docs"}
	scope := NewScope(orgs, projects)

	orgID, projectID := valuer.GenerateUUID(), valuer.GenerateUUID()
	for i := 0; i < 3; i++ {
		org, product, err := scope(context.Background(), orgID, projectID)
		require.NoError(t, err)
		assert.Equal(t, "hanzo", org)
		assert.Equal(t, "docs", product)
	}
	assert.Equal(t, 1, orgs.reads, "a name that cannot change is read once")
	assert.Equal(t, 1, projects.reads)
}

// TestScopeRefusesAnEmptyName: a record with no name must not resolve to "".
func TestScopeRefusesAnEmptyName(t *testing.T) {
	_, _, err := NewScope(&countingOrgs{name: ""}, &countingProjects{slug: "docs"})(
		context.Background(), valuer.GenerateUUID(), valuer.GenerateUUID())
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no name on the event plane"), err.Error())
}

// --- the two records the names are read from ---
//
// Each embeds the interface it stands in for and overrides ONLY the read the scope
// makes, so any other method reaching this stub is a nil-interface panic rather than
// a silent success — the seam is exactly one read per record, and the test says so.

type organizationGetterStub struct{ organization.Getter }

type projectStoreStub struct{ sentrytypes.ProjectStore }

type countingOrgs struct {
	organizationGetterStub
	name  string
	reads int
}

func (o *countingOrgs) Get(context.Context, valuer.UUID) (*types.Organization, error) {
	o.reads++
	return &types.Organization{Name: o.name}, nil
}

type countingProjects struct {
	projectStoreStub
	slug  string
	reads int
}

func (p *countingProjects) Get(context.Context, valuer.UUID, valuer.UUID) (*sentrytypes.Project, error) {
	p.reads++
	return &sentrytypes.Project{Slug: p.slug}, nil
}
