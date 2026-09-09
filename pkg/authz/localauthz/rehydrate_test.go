package localauthz

// A RESTART MUST NOT LOCK EVERYONE OUT.
//
// The tuple set is in-process. A grant writes BOTH the tuple and a durable
// user_role row, but only the row survives a restart — and the founding grant
// rides iamidentn's first sight of a user, whose ensureUser returns early once
// the durable user row exists. So after any restart (or on a second replica)
// every previously-seated principal held a durable role and no tuple, and
// ViewAccess/EditAccess/AdminAccess all answered 403 until someone
// re-provisioned them by hand. o11y runs one replica today, which is the only
// reason this had not yet been a full outage.
//
// These tests pin the repair at the seam that failed: a provider constructed
// fresh — exactly what a restart produces — authorizes a principal whose grant
// exists only in the durable rows.

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/instrumentation/instrumentationtest"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlstoretest"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

const adminRole = "o11y-admin"

// durable stands up a store whose user_role / service_account_role tables hold
// the given grants and nothing else — the state a process finds on boot.
func durable(t *testing.T, userID, serviceAccountID, orgID, roleID valuer.UUID) *sqlstoretest.Provider {
	t.Helper()
	store := sqlstoretest.New(sqlstore.Config{Provider: "sqlite"}, sqlmock.QueryMatcherRegexp)

	userRows := sqlmock.NewRows([]string{
		"id", "user_id", "role_id", "created_at", "updated_at",
		"role__id", "role__name", "role__description", "role__type", "role__org_id",
		"role__created_at", "role__updated_at",
	})
	if !userID.IsZero() {
		userRows.AddRow(
			valuer.GenerateUUID().StringValue(), userID.StringValue(), roleID.StringValue(), nil, nil,
			roleID.StringValue(), adminRole, "", "", orgID.StringValue(), nil, nil,
		)
	}
	store.Mock().ExpectQuery(`SELECT .* FROM "user_role"`).WillReturnRows(userRows)

	saRows := sqlmock.NewRows([]string{
		"id", "service_account_id", "role_id", "created_at", "updated_at",
		"role__id", "role__name", "role__description", "role__type", "role__org_id",
		"role__created_at", "role__updated_at",
	})
	if !serviceAccountID.IsZero() {
		saRows.AddRow(
			valuer.GenerateUUID().StringValue(), serviceAccountID.StringValue(), roleID.StringValue(), nil, nil,
			roleID.StringValue(), adminRole, "", "", orgID.StringValue(), nil, nil,
		)
	}
	store.Mock().ExpectQuery(`SELECT .* FROM "service_account_role"`).WillReturnRows(saRows)

	return store
}

func TestRestartRestoresGrantsFromDurableRows(t *testing.T) {
	ctx := context.Background()
	orgID, userID, roleID := valuer.GenerateUUID(), valuer.GenerateUUID(), valuer.GenerateUUID()

	p := newProvider(durable(t, userID, valuer.UUID{}, orgID, roleID), factory.NewScopedProviderSettings(instrumentationtest.New().ToProviderSettings(), "test")).(*provider)

	// The state a restart leaves behind: durable rows, empty tuple set.
	claims := authtypes.Claims{Principal: authtypes.PrincipalUser, UserID: userID.StringValue()}
	selectors := []coretypes.Selector{coretypes.TypeRole.MustSelector(adminRole)}
	if err := p.CheckWithTupleCreation(ctx, claims, orgID, authtypes.Relation{}, nil, nil, selectors); err == nil {
		t.Fatal("precondition: a fresh provider should hold no tuples before it starts")
	}

	if err := p.rehydrate(ctx); err != nil {
		t.Fatalf("rehydrate: %v", err)
	}

	if err := p.CheckWithTupleCreation(ctx, claims, orgID, authtypes.Relation{}, nil, nil, selectors); err != nil {
		t.Fatalf("a user holding %s in the durable rows is refused after a restart: %v", adminRole, err)
	}
}

// Service accounts are the callers that cannot notice a lockout and re-authenticate.
func TestRestartRestoresServiceAccountGrants(t *testing.T) {
	ctx := context.Background()
	orgID, serviceAccountID, roleID := valuer.GenerateUUID(), valuer.GenerateUUID(), valuer.GenerateUUID()

	p := newProvider(durable(t, valuer.UUID{}, serviceAccountID, orgID, roleID), factory.NewScopedProviderSettings(instrumentationtest.New().ToProviderSettings(), "test")).(*provider)
	if err := p.rehydrate(ctx); err != nil {
		t.Fatalf("rehydrate: %v", err)
	}

	claims := authtypes.Claims{
		Principal:        authtypes.PrincipalServiceAccount,
		ServiceAccountID: serviceAccountID.StringValue(),
	}
	selectors := []coretypes.Selector{coretypes.TypeRole.MustSelector(adminRole)}
	if err := p.CheckWithTupleCreation(ctx, claims, orgID, authtypes.Relation{}, nil, nil, selectors); err != nil {
		t.Fatalf("a service account holding %s in the durable rows is refused after a restart: %v", adminRole, err)
	}
}

// Rehydration reads what is STORED — it does not re-issue the founding grant. A
// principal whose role was revoked (no durable row) must stay revoked, or a
// restart would silently restore every removed admin.
func TestRehydrationDoesNotInventGrants(t *testing.T) {
	ctx := context.Background()
	orgID, roleID := valuer.GenerateUUID(), valuer.GenerateUUID()
	granted, revoked := valuer.GenerateUUID(), valuer.GenerateUUID()

	p := newProvider(durable(t, granted, valuer.UUID{}, orgID, roleID), factory.NewScopedProviderSettings(instrumentationtest.New().ToProviderSettings(), "test")).(*provider)
	if err := p.rehydrate(ctx); err != nil {
		t.Fatalf("rehydrate: %v", err)
	}

	selectors := []coretypes.Selector{coretypes.TypeRole.MustSelector(adminRole)}
	revokedClaims := authtypes.Claims{Principal: authtypes.PrincipalUser, UserID: revoked.StringValue()}
	if err := p.CheckWithTupleCreation(ctx, revokedClaims, orgID, authtypes.Relation{}, nil, nil, selectors); err == nil {
		t.Fatal("a user with no durable role row was authorized — rehydration invented a grant")
	}
}

// The org on the tuple comes from the ROLE's row, so a grant in one tenant
// cannot authorize the same subject in another.
func TestRehydratedGrantsStayInTheirOrg(t *testing.T) {
	ctx := context.Background()
	orgID, otherOrg := valuer.GenerateUUID(), valuer.GenerateUUID()
	userID, roleID := valuer.GenerateUUID(), valuer.GenerateUUID()

	p := newProvider(durable(t, userID, valuer.UUID{}, orgID, roleID), factory.NewScopedProviderSettings(instrumentationtest.New().ToProviderSettings(), "test")).(*provider)
	if err := p.rehydrate(ctx); err != nil {
		t.Fatalf("rehydrate: %v", err)
	}

	claims := authtypes.Claims{Principal: authtypes.PrincipalUser, UserID: userID.StringValue()}
	selectors := []coretypes.Selector{coretypes.TypeRole.MustSelector(adminRole)}
	if err := p.CheckWithTupleCreation(ctx, claims, otherOrg, authtypes.Relation{}, nil, nil, selectors); err == nil {
		t.Fatal("a grant rehydrated for one org authorized the subject in another")
	}
}

// THE LINK, not the function.
//
// Every test above calls p.rehydrate directly, and all four stayed GREEN when
// the call to it was deleted from Start — which is the whole defect they were
// written to prevent, reproduced one layer up. A rehydration nothing invokes is
// a lockout with a passing suite, and the thing the process actually runs is
// Start.
//
// So this drives START, on a provider whose only state is durable rows, and
// asks the question a request asks. Delete `p.rehydrate(ctx)` from Start and
// this goes red; the four above do not.
func TestStartRehydrates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	orgID, userID, roleID := valuer.GenerateUUID(), valuer.GenerateUUID(), valuer.GenerateUUID()
	p := newProvider(durable(t, userID, valuer.UUID{}, orgID, roleID), factory.NewScopedProviderSettings(instrumentationtest.New().ToProviderSettings(), "test"))

	started := make(chan error, 1)
	go func() { started <- p.Start(ctx) }()
	t.Cleanup(func() {
		_ = p.Stop(context.Background())
		<-started
	})

	claims := authtypes.Claims{Principal: authtypes.PrincipalUser, UserID: userID.StringValue()}
	selectors := []coretypes.Selector{coretypes.TypeRole.MustSelector(adminRole)}

	// Start blocks after rehydrating, so poll rather than sleep on a guess.
	deadline := time.Now().Add(4 * time.Second)
	var err error
	for time.Now().Before(deadline) {
		if err = p.CheckWithTupleCreation(ctx, claims, orgID, authtypes.Relation{}, nil, nil, selectors); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Start did not rehydrate the durable grants: %v", err)
}
