// Package roletypes exposes the role model used by authz and the managed-role
// seed migration. The role type and its constructors live in authtypes; this
// package re-exports them under a role-focused name and adds the storable-role
// conversion used when persisting managed roles.
package roletypes

import (
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/valuer"
)

// Role is the role model. It aliases authtypes.Role so values are
// interchangeable between the two packages.
type Role = authtypes.Role

// RoleTypeManaged marks roles that are seeded and owned by the platform and
// cannot be edited or deleted by users.
var RoleTypeManaged = authtypes.RoleTypeManaged

// Managed role names and descriptions, re-exported from authtypes.
var (
	HanzoO11yAdminRoleName            = authtypes.HanzoO11yAdminRoleName
	HanzoO11yAdminRoleDescription     = authtypes.HanzoO11yAdminRoleDescription
	HanzoO11yEditorRoleName           = authtypes.HanzoO11yEditorRoleName
	HanzoO11yEditorRoleDescription    = authtypes.HanzoO11yEditorRoleDescription
	HanzoO11yViewerRoleName           = authtypes.HanzoO11yViewerRoleName
	HanzoO11yViewerRoleDescription    = authtypes.HanzoO11yViewerRoleDescription
	HanzoO11yAnonymousRoleName        = authtypes.HanzoO11yAnonymousRoleName
	HanzoO11yAnonymousRoleDescription = authtypes.HanzoO11yAnonymousRoleDescription
)

// NewRole builds a role for the given org.
func NewRole(name, description string, roleType valuer.String, orgID valuer.UUID) *Role {
	return authtypes.NewRole(name, description, roleType, orgID)
}

// NewStorableRoleFromRole returns the role in the form persisted to the store.
// Role already embeds the bun model, so the value is storable as-is.
func NewStorableRoleFromRole(role *Role) *Role {
	return role
}

// MustGetHanzoO11yManagedRoleFromExistingRole maps a legacy role to its managed
// role name, panicking on an unknown role.
func MustGetHanzoO11yManagedRoleFromExistingRole(role types.Role) string {
	return authtypes.MustGetHanzoO11yManagedRoleFromExistingRole(role)
}
