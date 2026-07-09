package authtypes

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/uptrace/bun"
)

var (
	ErrCodeRoleInvalidInput                 = errors.MustNewCode("role_invalid_input")
	ErrCodeInvalidTypeRelation              = errors.MustNewCode("role_invalid_type_relation")
	ErrCodeRoleNotFound                     = errors.MustNewCode("role_not_found")
	ErrCodeRoleAlreadyExists                = errors.MustNewCode("role_already_exists")
	ErrCodeRoleFailedTransactionsFromString = errors.MustNewCode("role_failed_transactions_from_string")
	ErrCodeRoleUnsupported                  = errors.MustNewCode("role_unsupported")
	ErrCodeRoleHasUserAssignees             = errors.MustNewCode("role_has_user_assignees")
	ErrCodeRoleHasServiceAccountAssignees   = errors.MustNewCode("role_has_service_account_assignees")
	ErrCodeRoleHasAuthDomainMappings        = errors.MustNewCode("role_has_auth_domain_mappings")
)

var (
	roleNameRegex     = regexp.MustCompile("^[a-z-]{1,50}$")
	managedRolePrefix = "o11y"
)

var (
	RoleTypeCustom  = valuer.NewString("custom")
	RoleTypeManaged = valuer.NewString("managed")
)

var (
	O11yAnonymousRoleName        = coretypes.O11yAnonymousRoleName
	O11yAnonymousRoleDescription = "Role assigned to anonymous users for access to public resources."
	O11yAdminRoleName            = coretypes.O11yAdminRoleName
	O11yAdminRoleDescription     = "Role assigned to users who have full administrative access to O11y resources."
	O11yEditorRoleName           = coretypes.O11yEditorRoleName
	O11yEditorRoleDescription    = "Role assigned to users who can create, edit, and manage O11y resources but do not have full administrative privileges."
	O11yViewerRoleName           = coretypes.O11yViewerRoleName
	O11yViewerRoleDescription    = "Role assigned to users who have read-only access to O11y resources."
)

var (
	ExistingRoleToO11yManagedRoleMap = map[types.Role]string{
		types.RoleAdmin:  O11yAdminRoleName,
		types.RoleEditor: O11yEditorRoleName,
		types.RoleViewer: O11yViewerRoleName,
	}

	O11yManagedRoleToExistingLegacyRole = map[string]types.Role{
		O11yAdminRoleName:  types.RoleAdmin,
		O11yEditorRoleName: types.RoleEditor,
		O11yViewerRoleName: types.RoleViewer,
	}
)

type Role struct {
	bun.BaseModel `bun:"table:role"`

	types.Identifiable
	types.TimeAuditable
	Name        string        `bun:"name,type:string" json:"name" required:"true"`
	Description string        `bun:"description,type:string"  json:"description" required:"true"`
	Type        valuer.String `bun:"type,type:string" json:"type" required:"true"`
	OrgID       valuer.UUID   `bun:"org_id,type:string" json:"orgId" required:"true"`
}

type RoleWithTransactionGroups struct {
	*Role
	TransactionGroups TransactionGroups `json:"transactionGroups" required:"true" nullable:"false"`
}

type PostableRole struct {
	Name              string            `json:"name" required:"true"`
	Description       string            `json:"description" required:"false"`
	TransactionGroups TransactionGroups `json:"transactionGroups" required:"false" nullable:"false"`
}

type UpdatableRole struct {
	Description       string            `json:"description" required:"true"`
	TransactionGroups TransactionGroups `json:"transactionGroups" required:"true" nullable:"false"`
}

func NewRole(name, description string, roleType valuer.String, orgID valuer.UUID) *Role {
	return &Role{
		Identifiable: types.Identifiable{
			ID: valuer.GenerateUUID(),
		},
		TimeAuditable: types.TimeAuditable{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:        name,
		Description: description,
		Type:        roleType,
		OrgID:       orgID,
	}
}

func NewRoleWithTransactionGroups(name, description string, roleType valuer.String, orgID valuer.UUID, transactionGroups TransactionGroups) *RoleWithTransactionGroups {
	role := NewRole(name, description, roleType, orgID)

	return &RoleWithTransactionGroups{
		Role:              role,
		TransactionGroups: transactionGroups,
	}
}

func MakeRoleWithTransactionGroups(role *Role, transactionGroups TransactionGroups) *RoleWithTransactionGroups {
	return &RoleWithTransactionGroups{
		Role:              role,
		TransactionGroups: transactionGroups,
	}
}

func NewManagedRoles(orgID valuer.UUID) []*Role {
	return []*Role{
		NewRole(O11yAdminRoleName, O11yAdminRoleDescription, RoleTypeManaged, orgID),
		NewRole(O11yEditorRoleName, O11yEditorRoleDescription, RoleTypeManaged, orgID),
		NewRole(O11yViewerRoleName, O11yViewerRoleDescription, RoleTypeManaged, orgID),
		NewRole(O11yAnonymousRoleName, O11yAnonymousRoleDescription, RoleTypeManaged, orgID),
	}

}

func NewStatsFromRoles(roles []*Role) map[string]any {
	stats := make(map[string]any)
	for _, role := range roles {
		key := "role." + role.Type.StringValue() + ".count"
		if value, ok := stats[key]; ok {
			stats[key] = value.(int64) + 1
		} else {
			stats[key] = int64(1)
		}
	}
	stats["role.count"] = int64(len(roles))
	return stats
}

func (role *RoleWithTransactionGroups) Update(description string, transactionGroups TransactionGroups) error {
	err := role.ErrIfManaged()
	if err != nil {
		return err
	}

	role.Description = description
	role.TransactionGroups = transactionGroups
	role.UpdatedAt = time.Now()
	return nil
}

func (role *Role) ErrIfManaged() error {
	if role.Type == RoleTypeManaged {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeRoleInvalidInput, "cannot edit/delete managed role: %s", role.Name)
	}

	return nil
}

func (role *PostableRole) UnmarshalJSON(data []byte) error {
	shadow := struct {
		Name              string           `json:"name"`
		Description       string           `json:"description"`
		TransactionGroups *json.RawMessage `json:"transactionGroups"`
	}{}

	if err := json.Unmarshal(data, &shadow); err != nil {
		return err
	}

	if shadow.Name == "" {
		return errors.New(errors.TypeInvalidInput, ErrCodeRoleInvalidInput, "name is missing from the request")
	}

	if match := roleNameRegex.MatchString(shadow.Name); !match {
		return errors.New(errors.TypeInvalidInput, ErrCodeRoleInvalidInput, "name must contain only lowercase letters (a-z) and hyphens (-), and be at most 50 characters long.")
	}

	if strings.HasPrefix(shadow.Name, managedRolePrefix) {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeRoleInvalidInput, "role name cannot start with %q as it is reserved for O11y managed roles.", managedRolePrefix)
	}

	var transactionGroups TransactionGroups
	if shadow.TransactionGroups != nil {
		var err error
		transactionGroups, err = NewTransactionGroups(*shadow.TransactionGroups)
		if err != nil {
			return err
		}
	}

	role.Name = shadow.Name
	role.Description = shadow.Description
	role.TransactionGroups = transactionGroups
	return nil
}

func (role *UpdatableRole) UnmarshalJSON(data []byte) error {
	shadow := struct {
		Description       *string          `json:"description"`
		TransactionGroups *json.RawMessage `json:"transactionGroups"`
	}{}

	if err := json.Unmarshal(data, &shadow); err != nil {
		return err
	}

	if shadow.Description == nil {
		return errors.New(errors.TypeInvalidInput, ErrCodeRoleInvalidInput, "description is required").WithAdditional("send an empty string to clear the description")
	}

	if shadow.TransactionGroups == nil {
		return errors.New(errors.TypeInvalidInput, ErrCodeRoleInvalidInput, "transactionGroups is required").WithAdditional("send an empty array to clear the role's transaction groups")
	}

	transactionGroups, err := NewTransactionGroups(*shadow.TransactionGroups)
	if err != nil {
		return err
	}

	role.Description = *shadow.Description
	role.TransactionGroups = transactionGroups
	return nil
}

func MustGetO11yManagedRoleFromExistingRole(role types.Role) string {
	managedRole, ok := ExistingRoleToO11yManagedRoleMap[role]
	if !ok {
		panic(errors.Newf(errors.TypeInternal, errors.CodeInternal, "invalid role: %s", role.String()))
	}

	return managedRole
}

func NormalizeRoleName(role string) string {
	legacyRole, err := types.NewRole(strings.ToUpper(role))
	if err != nil {
		return role
	}

	managedRole, ok := ExistingRoleToO11yManagedRoleMap[legacyRole]
	if !ok {
		return role
	}

	return managedRole
}

type RoleStore interface {
	Create(context.Context, *Role) error
	Get(context.Context, valuer.UUID, valuer.UUID) (*Role, error)
	GetByOrgIDAndName(context.Context, valuer.UUID, string) (*Role, error)
	List(context.Context, valuer.UUID) ([]*Role, error)
	ListByOrgIDAndNames(context.Context, valuer.UUID, []string) ([]*Role, error)
	ListByOrgIDAndIDs(context.Context, valuer.UUID, []valuer.UUID) ([]*Role, error)
	Update(context.Context, valuer.UUID, *Role) error
	Delete(context.Context, valuer.UUID, valuer.UUID) error
	RunInTx(context.Context, func(ctx context.Context) error) error
}
