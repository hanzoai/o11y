package impluser

import (
	"context"
	"time"

	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// The root-user service seeds the ONE row that local single-user mode stands in
// for: identn::impersonation resolves every request to it, which is the whole of
// that mode and the reason it refuses to boot beside any real resolver.
//
// It used to reconcile a PASSWORD onto that row on every start — create it,
// compare it, rotate it — from `user::root::password` in the process
// environment. Nothing reads a password any more, so there is nothing to
// reconcile: the row's existence IS the whole fact. What is left is
// resolve-or-create the org with its managed roles, and resolve-or-create the
// root row with the admin role, which is the same shape iamidentn uses when a
// real IAM subject arrives.
type service struct {
	settings  factory.ScopedProviderSettings
	store     types.UserStore
	setter    user.Setter
	orgGetter organization.Getter
	orgSetter organization.Setter
	authz     authz.AuthZ
	config    user.RootConfig
	stopC     chan struct{}
	healthyC  chan struct{}
}

func NewService(
	providerSettings factory.ProviderSettings,
	store types.UserStore,
	setter user.Setter,
	orgGetter organization.Getter,
	orgSetter organization.Setter,
	authz authz.AuthZ,
	config user.RootConfig,
) user.Service {
	return &service{
		settings:  factory.NewScopedProviderSettings(providerSettings, "go.o11y.io/pkg/modules/user"),
		store:     store,
		setter:    setter,
		orgGetter: orgGetter,
		orgSetter: orgSetter,
		authz:     authz,
		config:    config,
		stopC:     make(chan struct{}),
		healthyC:  make(chan struct{}),
	}
}

func (s *service) Start(ctx context.Context) error {
	if !s.config.Enabled {
		close(s.healthyC)
		<-s.stopC
		return nil
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		err := s.reconcile(ctx)
		if err == nil {
			s.settings.Logger().InfoContext(ctx, "root user reconciliation completed successfully")
			close(s.healthyC)
			<-s.stopC
			return nil
		}

		s.settings.Logger().WarnContext(ctx, "root user reconciliation failed, retrying", errors.Attr(err))

		select {
		case <-s.stopC:
			return nil
		case <-ticker.C:
			continue
		}
	}
}

func (s *service) Healthy() <-chan struct{} { return s.healthyC }

func (s *service) Stop(_ context.Context) error { close(s.stopC); return nil }

func (s *service) reconcile(ctx context.Context) error {
	org, resolvedByName, err := s.orgGetter.GetByIDOrName(ctx, s.config.Org.ID, s.config.Org.Name)
	if err != nil {
		if !errors.Ast(err, errors.TypeNotFound) {
			return err
		}

		newOrg := types.NewOrganization(s.config.Org.Name, s.config.Org.Name)
		if !s.config.Org.ID.IsZero() {
			newOrg = types.NewOrganizationWithID(s.config.Org.ID, s.config.Org.Name, s.config.Org.Name)
		}

		managedRoles := authtypes.NewManagedRoles(newOrg.ID)
		if err := s.orgSetter.Create(ctx, newOrg, func(ctx context.Context, id valuer.UUID) error {
			return s.authz.CreateManagedRoles(ctx, id, managedRoles)
		}); err != nil && !errors.Ast(err, errors.TypeAlreadyExists) {
			return err
		}

		return s.ensureRootUser(ctx, newOrg.ID)
	}

	if !s.config.Org.ID.IsZero() && resolvedByName {
		// the existing org has the same name as config but org id is different; inform user with actionable message
		return errors.Newf(errors.TypeInvalidInput, errors.CodeInvalidInput, "organization with name %q already exists with a different ID %s (expected %s)", s.config.Org.Name, org.ID.StringValue(), s.config.Org.ID.StringValue())
	}

	return s.ensureRootUser(ctx, org.ID)
}

// ensureRootUser creates the root row on first sight, with the org's admin role.
// A row that is already there is already the whole fact — there is nothing left
// to bring into line now that it carries no credential.
func (s *service) ensureRootUser(ctx context.Context, orgID valuer.UUID) error {
	existing, err := s.store.GetRootUserByOrgID(ctx, orgID)
	if err != nil && !errors.Ast(err, errors.TypeNotFound) {
		return err
	}
	if existing != nil {
		return nil
	}

	rootUser, err := types.NewRootUser(s.config.Email.String(), s.config.Email, orgID)
	if err != nil {
		return err
	}

	if err := s.setter.CreateUser(ctx, rootUser, user.WithRoleNames([]string{authtypes.O11yAdminRoleName})); err != nil && !errors.Ast(err, errors.TypeAlreadyExists) {
		return err
	}

	return nil
}
