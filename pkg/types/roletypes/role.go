package roletypes

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/uptrace/bun"
)

var (
	ErrCodeRoleInvalidInput                 = errors.MustNewCode("role_invalid_input")
	ErrCodeRoleEmptyPatch                   = errors.MustNewCode("role_empty_patch")
	ErrCodeInvalidTypeRelation              = errors.MustNewCode("role_invalid_type_relation")
	ErrCodeRoleNotFound                     = errors.MustNewCode("role_not_found")
	ErrCodeRoleFailedTransactionsFromString = errors.MustNewCode("role_failed_transactions_from_string")
	ErrCodeRoleUnsupported                  = errors.MustNewCode("role_unsupported")
)

var (
	roleNameRegex = regexp.MustCompile("^[a-z-]{1,50}$")
)

var (
	RoleTypeCustom  = valuer.NewString("custom")
	RoleTypeManaged = valuer.NewString("managed")
)

var (
	Hanzo O11yAnonymousRoleName        = "o11y-anonymous"
	Hanzo O11yAnonymousRoleDescription = "Role assigned to anonymous users for access to public resources."
	Hanzo O11yAdminRoleName            = "o11y-admin"
	Hanzo O11yAdminRoleDescription     = "Role assigned to users who have full administrative access to Hanzo O11y resources."
	Hanzo O11yEditorRoleName           = "o11y-editor"
	Hanzo O11yEditorRoleDescription    = "Role assigned to users who can create, edit, and manage Hanzo O11y resources but do not have full administrative privileges."
	Hanzo O11yViewerRoleName           = "o11y-viewer"
	Hanzo O11yViewerRoleDescription    = "Role assigned to users who have read-only access to Hanzo O11y resources."
)

var (
	ExistingRoleToHanzo O11yManagedRoleMap = map[types.Role]string{
		types.RoleAdmin:  Hanzo O11yAdminRoleName,
		types.RoleEditor: Hanzo O11yEditorRoleName,
		types.RoleViewer: Hanzo O11yViewerRoleName,
	}
)

var (
	TypeableResourcesRoles = authtypes.MustNewTypeableMetaResources(authtypes.MustNewName("roles"))
)

type StorableRole struct {
	bun.BaseModel `bun:"table:role"`

	types.Identifiable
	types.TimeAuditable
	Name        string `bun:"name,type:string"`
	Description string `bun:"description,type:string"`
	Type        string `bun:"type,type:string"`
	OrgID       string `bun:"org_id,type:string"`
}

type Role struct {
	types.Identifiable
	types.TimeAuditable
	Name        string        `json:"name" required:"true"`
	Description string        `json:"description" required:"true"`
	Type        valuer.String `json:"type" required:"true"`
	OrgID       valuer.UUID   `json:"orgId" required:"true"`
}

type PostableRole struct {
	Name        string `json:"name" required:"true"`
	Description string `json:"description"`
}

type PatchableRole struct {
	Description string `json:"description" required:"true"`
}

func NewStorableRoleFromRole(role *Role) *StorableRole {
	return &StorableRole{
		Identifiable:  role.Identifiable,
		TimeAuditable: role.TimeAuditable,
		Name:          role.Name,
		Description:   role.Description,
		Type:          role.Type.String(),
		OrgID:         role.OrgID.StringValue(),
	}
}

func NewRoleFromStorableRole(storableRole *StorableRole) *Role {
	return &Role{
		Identifiable:  storableRole.Identifiable,
		TimeAuditable: storableRole.TimeAuditable,
		Name:          storableRole.Name,
		Description:   storableRole.Description,
		Type:          valuer.NewString(storableRole.Type),
		OrgID:         valuer.MustNewUUID(storableRole.OrgID),
	}
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
		NewRole(Hanzo O11yAdminRoleName, Hanzo O11yAdminRoleDescription, RoleTypeManaged, orgID),
		NewRole(Hanzo O11yEditorRoleName, Hanzo O11yEditorRoleDescription, RoleTypeManaged, orgID),
		NewRole(Hanzo O11yViewerRoleName, Hanzo O11yViewerRoleDescription, RoleTypeManaged, orgID),
		NewRole(Hanzo O11yAnonymousRoleName, Hanzo O11yAnonymousRoleDescription, RoleTypeManaged, orgID),
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
		return errors.Newf(errors.TypeInvalidInput, ErrCodeRoleInvalidInput, "name must conform to the regex: %s", roleNameRegex.String())
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

func GetAdditionTuples(name string, orgID valuer.UUID, relation authtypes.Relation, additions []*authtypes.Object) ([]*openfgav1.TupleKey, error) {
	tuples := make([]*openfgav1.TupleKey, 0)

	for _, object := range additions {
		typeable := authtypes.MustNewTypeableFromType(object.Resource.Type, object.Resource.Name)
		transactionTuples, err := typeable.Tuples(
			authtypes.MustNewSubject(
				authtypes.TypeableRole,
				name,
				orgID,
				&authtypes.RelationAssignee,
			),
			relation,
			[]authtypes.Selector{object.Selector},
			orgID,
		)
		if err != nil {
			return nil, err
		}

		tuples = append(tuples, transactionTuples...)
	}

	return tuples, nil
}

func GetDeletionTuples(name string, orgID valuer.UUID, relation authtypes.Relation, deletions []*authtypes.Object) ([]*openfgav1.TupleKey, error) {
	tuples := make([]*openfgav1.TupleKey, 0)

	for _, object := range deletions {
		typeable := authtypes.MustNewTypeableFromType(object.Resource.Type, object.Resource.Name)
		transactionTuples, err := typeable.Tuples(
			authtypes.MustNewSubject(
				authtypes.TypeableRole,
				name,
				orgID,
				&authtypes.RelationAssignee,
			),
			relation,
			[]authtypes.Selector{object.Selector},
			orgID,
		)
		if err != nil {
			return nil, err
		}

		tuples = append(tuples, transactionTuples...)
	}

	return tuples, nil
}

func MustGetHanzo O11yManagedRoleFromExistingRole(role types.Role) string {
	managedRole, ok := ExistingRoleToHanzo O11yManagedRoleMap[role]
	if !ok {
		panic(errors.Newf(errors.TypeInternal, errors.CodeInternal, "invalid role: %s", role.String()))
	}

	return managedRole
}
