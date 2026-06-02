package authtypes

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/uptrace/bun"
)

var (
	ErrCodeRoleInvalidInput                 = errors.MustNewCode("role_invalid_input")
	ErrCodeRoleEmptyPatch                   = errors.MustNewCode("role_empty_patch")
	ErrCodeInvalidTypeRelation              = errors.MustNewCode("role_invalid_type_relation")
	ErrCodeRoleNotFound                     = errors.MustNewCode("role_not_found")
	ErrCodeRoleFailedTransactionsFromString = errors.MustNewCode("role_failed_transactions_from_string")
	ErrCodeRoleUnsupported                  = errors.MustNewCode("role_unsupported")
	ErrCodeRoleHasUserAssignees             = errors.MustNewCode("role_has_user_assignees")
	ErrCodeRoleHasServiceAccountAssignees   = errors.MustNewCode("role_has_service_account_assignees")
)

var (
	roleNameRegex     = regexp.MustCompile("^[a-z-]{1,50}$")
	managedRolePrefix = "signoz"
)

var (
	RoleTypeCustom  = valuer.NewString("custom")
	RoleTypeManaged = valuer.NewString("managed")
)

var (
	HanzoO11yAnonymousRoleName        = "o11y-anonymous"
	HanzoO11yAnonymousRoleDescription = "Role assigned to anonymous users for access to public resources."
	HanzoO11yAdminRoleName            = "o11y-admin"
	HanzoO11yAdminRoleDescription     = "Role assigned to users who have full administrative access to HanzoO11y resources."
	HanzoO11yEditorRoleName           = "o11y-editor"
	HanzoO11yEditorRoleDescription    = "Role assigned to users who can create, edit, and manage HanzoO11y resources but do not have full administrative privileges."
	HanzoO11yViewerRoleName           = "o11y-viewer"
	HanzoO11yViewerRoleDescription    = "Role assigned to users who have read-only access to HanzoO11y resources."
)

var (
	ExistingRoleToHanzoO11yManagedRoleMap = map[types.Role]string{
		types.RoleAdmin:  HanzoO11yAdminRoleName,
		types.RoleEditor: HanzoO11yEditorRoleName,
		types.RoleViewer: HanzoO11yViewerRoleName,
	}

	SigNozManagedRoleToExistingLegacyRole = map[string]types.Role{
		SigNozAdminRoleName:  types.RoleAdmin,
		SigNozEditorRoleName: types.RoleEditor,
		SigNozViewerRoleName: types.RoleViewer,
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

type PostableRole struct {
	Name        string `json:"name" required:"true"`
	Description string `json:"description"`
}

type PatchableRole struct {
	Description string `json:"description" required:"true"`
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

func NewManagedRoles(orgID valuer.UUID) []*Role {
	return []*Role{
		NewRole(HanzoO11yAdminRoleName, HanzoO11yAdminRoleDescription, RoleTypeManaged, orgID),
		NewRole(HanzoO11yEditorRoleName, HanzoO11yEditorRoleDescription, RoleTypeManaged, orgID),
		NewRole(HanzoO11yViewerRoleName, HanzoO11yViewerRoleDescription, RoleTypeManaged, orgID),
		NewRole(HanzoO11yAnonymousRoleName, HanzoO11yAnonymousRoleDescription, RoleTypeManaged, orgID),
	}

}

func (role *Role) PatchMetadata(description string) error {
	err := role.ErrIfManaged()
	if err != nil {
		return err
	}

	role.Description = description
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
	type shadowPostableRole struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	var shadowRole shadowPostableRole
	if err := json.Unmarshal(data, &shadowRole); err != nil {
		return err
	}

	if shadowRole.Name == "" {
		return errors.New(errors.TypeInvalidInput, ErrCodeRoleInvalidInput, "name is missing from the request")
	}

	if match := roleNameRegex.MatchString(shadowRole.Name); !match {
		return errors.New(errors.TypeInvalidInput, ErrCodeRoleInvalidInput, "name must contain only lowercase letters (a-z) and hyphens (-), and be at most 50 characters long.")
	}

	if strings.HasPrefix(shadowRole.Name, managedRolePrefix) {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeRoleInvalidInput, "role name cannot start with %q as it is reserved for SigNoz managed roles.", managedRolePrefix)
	}

	role.Name = shadowRole.Name
	role.Description = shadowRole.Description

	return nil
}

func (role *PatchableRole) UnmarshalJSON(data []byte) error {
	type shadowPatchableRole struct {
		Description string `json:"description"`
	}

	var shadowRole shadowPatchableRole
	if err := json.Unmarshal(data, &shadowRole); err != nil {
		return err
	}

	if shadowRole.Description == "" {
		return errors.New(errors.TypeInvalidInput, ErrCodeRoleEmptyPatch, "empty role patch request received, description must be present")
	}

	role.Description = shadowRole.Description

	return nil
}

func GetAdditionTuples(name string, orgID valuer.UUID, relation authtypes.Relation, additions []*authtypes.Object) ([]*authtypes.TupleKey, error) {
	tuples := make([]*authtypes.TupleKey, 0)

	for _, object := range additions {
		resource := coretypes.MustNewResourceFromTypeAndKind(object.Resource.Type, object.Resource.Kind)
		transactionTuples := NewTuples(
			resource,
			MustNewSubject(
				coretypes.NewResourceRole(),
				name,
				orgID,
				&coretypes.VerbAssignee,
			),
			relation,
			[]coretypes.Selector{object.Selector},
			orgID,
		)

		tuples = append(tuples, transactionTuples...)
	}

	return tuples, nil
}

func GetDeletionTuples(name string, orgID valuer.UUID, relation authtypes.Relation, deletions []*authtypes.Object) ([]*authtypes.TupleKey, error) {
	tuples := make([]*authtypes.TupleKey, 0)

	for _, object := range deletions {
		resource := coretypes.MustNewResourceFromTypeAndKind(object.Resource.Type, object.Resource.Kind)
		transactionTuples := NewTuples(
			resource,
			MustNewSubject(
				coretypes.NewResourceRole(),
				name,
				orgID,
				&coretypes.VerbAssignee,
			),
			relation,
			[]coretypes.Selector{object.Selector},
			orgID,
		)

		tuples = append(tuples, transactionTuples...)
	}

	return tuples, nil
}

func MustGetHanzoO11yManagedRoleFromExistingRole(role types.Role) string {
	managedRole, ok := ExistingRoleToHanzoO11yManagedRoleMap[role]
	if !ok {
		panic(errors.Newf(errors.TypeInternal, errors.CodeInternal, "invalid role: %s", role.String()))
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
