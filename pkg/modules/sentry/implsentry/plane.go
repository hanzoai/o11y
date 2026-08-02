package implsentry

import (
	"context"
	"sync"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// THE PLANE'S VOCABULARY, said once.
//
// `event.error` is a slice of the ONE event plane, and the plane names a tenant by its
// IAM organization SLUG and a surface by its PRODUCT — the strings every other writer
// on it emits (`hanzo`, `docs`) and every reader binds. This module works in UUIDs,
// because that is what the HTTP surface and the project rows are keyed on. The two
// vocabularies meet HERE and nowhere else.
//
// They used not to meet at all: Insert wrote `orgID.String()` straight into `org`, so
// ONE tenant appeared on the plane as two tenants — 86 rows under
// cd35a51b-f7e7-5412-b67f-58b8703e5219 and 2 under `hanzo`, which is the UUIDv5 of
// exactly that slug. A reader binding either spelling saw a fraction of its own
// errors, and the plane's own tenant assertion (0002, step 5) rejects the column.
//
// WHY THE SLUG AND NOT THE UUID:
//
//   - The uuid is DERIVED FROM the slug. `iamidentn.toUUID` is
//     uuid5(NameSpaceURL, "hanzo:o11y:org:"+slug), so the slug is the primal value and
//     the uuid a projection of it. Storing the projection loses the original and makes
//     every reader re-derive; storing the primal value is total.
//   - Every other writer and reader on the plane already says slug: cloud stamps the
//     validated IAM owner into `org`, cloud's lenses bind `org = ?` with it, insights
//     routes on it, llmobs filters on it. A second spelling is not a second opinion,
//     it is a partition.
//   - The plane's DDL says so: "`org` is THE tenant: the IAM organization slug,
//     stamped server-side from the validated principal, first in every sort key in
//     this namespace. Never a UUID, never an integer, never translated through a
//     lookup table."
//   - `org` is LowCardinality(String) and leads every sort key here. A uuid defeats
//     both the dictionary and the property that a row says whose it is.
//
// The same argument decides `product`: a Sentry project IS a surface — `docs`, `app`,
// `chat` — so its slug is the product name, and the plane already holds those exact
// values from cloud. A project's slug is assigned at creation and there is no update
// path for it (sentry.Module has Create/Get/List/Rotate/Delete and no Update), so it
// is stable enough to key rows on.
//
// A CONSEQUENCE WORTH SAYING OUT LOUD: because cloud writes `product = 'docs'` for the
// same org, a Sentry project slugged `docs` now reads cloud's errors for that surface
// as well as its own. That is the one table doing what it is for — same tenant, same
// surface, same kind of fact — and it cannot cross a tenant, because `org` leads every
// predicate. It is a UNION, not a leak. If a project ever needs to see only what its
// own DSN captured, that is a narrower on how the fact ARRIVED, and the honest place
// for it is a column that says so, not a second spelling of the surface.

// Scope resolves the identifiers a caller works in to the names the plane stores. It
// is the ONE place a uuid becomes a name; everything above it speaks uuid and
// everything below it speaks the plane.
type Scope func(ctx context.Context, orgID, projectID valuer.UUID) (org, product string, err error)

// NewScope reads the names from the records that own them — the org row and the
// project row — and memoizes them.
//
// Memoizing is sound because neither name changes: an org's is written once by
// provisioning and a project's once by Create. It matters because Scope is on the
// ingest path, where the alternative is two row reads per batch.
func NewScope(orgs organization.Getter, projects sentrytypes.ProjectStore) Scope {
	var known sync.Map // valuer.UUID -> string, for both kinds; ids are globally unique

	name := func(id valuer.UUID, read func() (string, error)) (string, error) {
		if cached, ok := known.Load(id); ok {
			return cached.(string), nil
		}
		got, err := read()
		if err != nil {
			return "", err
		}
		// An empty name is NOT cached and NOT returned: naming is the tenant
		// boundary, so an unnamed id must fail the operation rather than write a
		// row nobody owns.
		if got == "" {
			return "", errors.Newf(errors.TypeNotFound, sentrytypes.ErrCodeSentryNotFound,
				"no name on the event plane for %s", id.String())
		}
		known.Store(id, got)
		return got, nil
	}

	return func(ctx context.Context, orgID, projectID valuer.UUID) (string, string, error) {
		org, err := name(orgID, func() (string, error) {
			row, err := orgs.Get(ctx, orgID)
			if err != nil {
				return "", err
			}
			return row.Name, nil
		})
		if err != nil {
			return "", "", err
		}
		product, err := name(projectID, func() (string, error) {
			row, err := projects.Get(ctx, orgID, projectID)
			if err != nil {
				return "", err
			}
			return row.Slug, nil
		})
		if err != nil {
			return "", "", err
		}
		return org, product, nil
	}
}
