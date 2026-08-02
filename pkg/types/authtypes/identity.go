package authtypes

import (
	"encoding/json"

	"github.com/hanzoai/o11y/pkg/valuer"
)

// Identity is WHO a request is, as resolved by an identN provider. It is
// resolved, never issued: o11y reads it out of what the edge asserted (the
// gateway's identity headers), or out of a service-account API key, and turns
// it into the Claims every handler downstream reads.
//
// This file is what survived pkg/types/authtypes/authn.go. The rest of that
// file described how o11y AUTHENTICATED people — providers named
// email_password / google_auth / saml / oidc, a callback identity carried
// through an OAuth `state`, an AuthNStore that read a user's stored password.
// o11y authenticates nobody now; Hanzo IAM does, once, at the edge.
var (
	PrincipalUser           = Principal{valuer.NewString("user")}
	PrincipalServiceAccount = Principal{valuer.NewString("service_account")}
)

// Principal is WHAT KIND of thing the identity is — a person or a machine. It
// is the one axis authorization branches on before it looks at roles.
type Principal struct{ valuer.String }

type Identity struct {
	UserID           valuer.UUID    `json:"userId"`
	ServiceAccountID valuer.UUID    `json:"serviceAccountId"`
	Principal        Principal      `json:"principal"`
	OrgID            valuer.UUID    `json:"orgId"`
	IdenNProvider    IdentNProvider `json:"identNProvider"`
	Email            valuer.Email   `json:"email"`
}

func NewIdentity(userID valuer.UUID, serviceAccountID valuer.UUID, principal Principal, orgID valuer.UUID, email valuer.Email, identNProvider IdentNProvider) *Identity {
	return &Identity{
		UserID:           userID,
		ServiceAccountID: serviceAccountID,
		Principal:        principal,
		OrgID:            orgID,
		Email:            email,
		IdenNProvider:    identNProvider,
	}
}

func NewPrincipalUserIdentity(userID valuer.UUID, orgID valuer.UUID, email valuer.Email, identNProvider IdentNProvider) *Identity {
	return &Identity{
		UserID:        userID,
		Principal:     PrincipalUser,
		OrgID:         orgID,
		Email:         email,
		IdenNProvider: identNProvider,
	}
}

func NewPrincipalServiceAccountIdentity(serviceAccountID valuer.UUID, orgID valuer.UUID, email valuer.Email, identNProvider IdentNProvider) *Identity {
	return &Identity{
		ServiceAccountID: serviceAccountID,
		Principal:        PrincipalServiceAccount,
		OrgID:            orgID,
		Email:            email,
		IdenNProvider:    identNProvider,
	}
}

func (typ Identity) MarshalBinary() ([]byte, error) { return json.Marshal(typ) }

func (typ *Identity) UnmarshalBinary(data []byte) error { return json.Unmarshal(data, typ) }

func (typ *Identity) ToClaims() Claims {
	return Claims{
		UserID:           typ.UserID.String(),
		ServiceAccountID: typ.ServiceAccountID.String(),
		Principal:        typ.Principal,
		Email:            typ.Email.String(),
		OrgID:            typ.OrgID.String(),
		IdentNProvider:   typ.IdenNProvider,
	}
}
