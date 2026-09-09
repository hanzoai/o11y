package user

// The only thing a caller still says about a user it creates is which roles the
// user holds. WithFactorPassword is gone with the password store, WithRoleIDs
// with the member-administration API, and the authenticate options with the
// sessions o11y no longer mints.
type createUserOptions struct {
	RoleNames []string
}

type CreateUserOption func(*createUserOptions)

func WithRoleNames(roleNames []string) CreateUserOption {
	return func(o *createUserOptions) {
		o.RoleNames = roleNames
	}
}

func NewCreateUserOptions(opts ...CreateUserOption) *createUserOptions {
	o := &createUserOptions{RoleNames: nil}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
