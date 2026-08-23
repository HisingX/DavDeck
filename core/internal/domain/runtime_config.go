package domain

// ShareWithPermissions is the canonical immutable ACL snapshot for one share.
type ShareWithPermissions struct {
	Share       Share
	Permissions []SharePermission
}

// RuntimeConfigInput is the canonical desired-state snapshot consumed by the
// Caddy compiler. It contains no persistence or platform behavior.
type RuntimeConfigInput struct {
	ServerSettings ServerSettings
	TLSProfile     *TLSProfile
	Users          []User
	Shares         []ShareWithPermissions
}
