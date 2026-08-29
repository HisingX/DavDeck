package domain

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ConfigRevisionSnapshotVersion identifies the private, database-only state
// snapshot stored with a generated configuration revision. It intentionally
// includes password hashes so a rollback can restore deleted users without
// ever storing plaintext passwords.
const ConfigRevisionSnapshotVersion = 1

// ConfigRevisionSnapshot is the complete desired application state needed to
// restore a revision consistently with its generated Caddy configuration.
// It is never exposed through the Management API.
type ConfigRevisionSnapshot struct {
	Version        int                           `json:"version"`
	ServerSettings ServerSettings                `json:"server_settings"`
	TLSProfile     *TLSProfile                   `json:"tls_profile,omitempty"`
	Users          []configRevisionSnapshotUser  `json:"users"`
	Shares         []configRevisionSnapshotShare `json:"shares"`
}

type configRevisionSnapshotUser struct {
	ID                 ID        `json:"id"`
	Username           string    `json:"username"`
	UsernameNormalized string    `json:"username_normalized"`
	PasswordHash       string    `json:"password_hash"`
	Enabled            bool      `json:"enabled"`
	CreatedAt          Timestamp `json:"created_at"`
	UpdatedAt          Timestamp `json:"updated_at"`
}

type configRevisionSnapshotShare struct {
	Share       Share             `json:"share"`
	Permissions []SharePermission `json:"permissions"`
}

// NewConfigRevisionSnapshot creates an immutable copy of the desired state.
func NewConfigRevisionSnapshot(input RuntimeConfigInput) ConfigRevisionSnapshot {
	users := make([]configRevisionSnapshotUser, 0, len(input.Users))
	for _, user := range input.Users {
		users = append(users, configRevisionSnapshotUser{
			ID: user.ID, Username: user.Username, UsernameNormalized: user.UsernameNormalized,
			PasswordHash: user.PasswordHash, Enabled: user.Enabled,
			CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
		})
	}
	shares := make([]configRevisionSnapshotShare, 0, len(input.Shares))
	for _, item := range input.Shares {
		permissions := append([]SharePermission(nil), item.Permissions...)
		sort.Slice(permissions, func(i, j int) bool {
			return permissions[i].UserID < permissions[j].UserID
		})
		shares = append(shares, configRevisionSnapshotShare{
			Share: item.Share, Permissions: permissions,
		})
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].UsernameNormalized == users[j].UsernameNormalized {
			return users[i].ID < users[j].ID
		}
		return users[i].UsernameNormalized < users[j].UsernameNormalized
	})
	sort.Slice(shares, func(i, j int) bool {
		if shares[i].Share.Slug == shares[j].Share.Slug {
			return shares[i].Share.ID < shares[j].Share.ID
		}
		return shares[i].Share.Slug < shares[j].Share.Slug
	})
	var tlsProfile *TLSProfile
	if input.TLSProfile != nil {
		profile := *input.TLSProfile
		tlsProfile = &profile
	}
	return ConfigRevisionSnapshot{
		Version: ConfigRevisionSnapshotVersion, ServerSettings: input.ServerSettings,
		TLSProfile: tlsProfile, Users: users, Shares: shares,
	}
}

// RuntimeConfigInput returns the canonical domain state represented by the
// snapshot. Call Validate on the snapshot before using this method.
func (s ConfigRevisionSnapshot) RuntimeConfigInput() RuntimeConfigInput {
	users := make([]User, 0, len(s.Users))
	for _, user := range s.Users {
		users = append(users, User{
			ID: user.ID, Username: user.Username, UsernameNormalized: user.UsernameNormalized,
			PasswordHash: user.PasswordHash, Enabled: user.Enabled,
			CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
		})
	}
	shares := make([]ShareWithPermissions, 0, len(s.Shares))
	for _, item := range s.Shares {
		shares = append(shares, ShareWithPermissions{
			Share: item.Share, Permissions: append([]SharePermission(nil), item.Permissions...),
		})
	}
	var tlsProfile *TLSProfile
	if s.TLSProfile != nil {
		profile := *s.TLSProfile
		tlsProfile = &profile
	}
	return RuntimeConfigInput{
		ServerSettings: s.ServerSettings, TLSProfile: tlsProfile,
		Users: users, Shares: shares,
	}
}

// Validate checks that the snapshot is a complete, internally consistent
// desired-state document before it is written back to SQLite.
func (s ConfigRevisionSnapshot) Validate() error {
	if s.Version != ConfigRevisionSnapshotVersion {
		return fmt.Errorf("unsupported revision snapshot version %d", s.Version)
	}
	return s.RuntimeConfigInput().Validate()
}

// MarshalConfigRevisionSnapshot serializes and validates a private revision
// snapshot. The returned bytes must not be sent to a client or written to a
// log because they contain password hashes.
func MarshalConfigRevisionSnapshot(input RuntimeConfigInput) ([]byte, error) {
	snapshot := NewConfigRevisionSnapshot(input)
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(snapshot)
}

// ParseConfigRevisionSnapshot parses a private revision snapshot from the
// database and returns validated desired state.
func ParseConfigRevisionSnapshot(body []byte) (RuntimeConfigInput, error) {
	if len(body) == 0 {
		return RuntimeConfigInput{}, fmt.Errorf("revision has no state snapshot")
	}
	var snapshot ConfigRevisionSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return RuntimeConfigInput{}, fmt.Errorf("decode revision state snapshot: %w", err)
	}
	if err := snapshot.Validate(); err != nil {
		return RuntimeConfigInput{}, fmt.Errorf("validate revision state snapshot: %w", err)
	}
	return snapshot.RuntimeConfigInput(), nil
}

// Validate checks the complete desired state represented by a snapshot.
func (input RuntimeConfigInput) Validate() error {
	if err := input.ServerSettings.Validate(); err != nil {
		return fmt.Errorf("server settings: %w", err)
	}
	if input.TLSProfile != nil {
		if err := input.TLSProfile.Validate(); err != nil {
			return fmt.Errorf("TLS profile: %w", err)
		}
	}
	users := make(map[ID]struct{}, len(input.Users))
	normalizedUsers := make(map[string]struct{}, len(input.Users))
	for _, user := range input.Users {
		if err := user.Validate(); err != nil {
			return fmt.Errorf("user %s: %w", user.ID, err)
		}
		if _, exists := users[user.ID]; exists {
			return fmt.Errorf("duplicate user id %s", user.ID)
		}
		if _, exists := normalizedUsers[user.UsernameNormalized]; exists {
			return fmt.Errorf("duplicate normalized username %s", user.UsernameNormalized)
		}
		users[user.ID] = struct{}{}
		normalizedUsers[user.UsernameNormalized] = struct{}{}
	}
	shares := make(map[ID]struct{}, len(input.Shares))
	slugs := make(map[string]struct{}, len(input.Shares))
	for _, item := range input.Shares {
		if err := item.Share.Validate(); err != nil {
			return fmt.Errorf("share %s: %w", item.Share.ID, err)
		}
		if _, exists := shares[item.Share.ID]; exists {
			return fmt.Errorf("duplicate share id %s", item.Share.ID)
		}
		if _, exists := slugs[item.Share.Slug]; exists {
			return fmt.Errorf("duplicate share slug %s", item.Share.Slug)
		}
		shares[item.Share.ID] = struct{}{}
		slugs[item.Share.Slug] = struct{}{}
		permissions := make(map[ID]struct{}, len(item.Permissions))
		for _, permission := range item.Permissions {
			if err := permission.Validate(); err != nil {
				return fmt.Errorf("share %s permission: %w", item.Share.ID, err)
			}
			if permission.ShareID != item.Share.ID {
				return fmt.Errorf("permission references a different share")
			}
			if _, exists := users[permission.UserID]; !exists {
				return fmt.Errorf("permission references unknown user %s", permission.UserID)
			}
			if _, exists := permissions[permission.UserID]; exists {
				return fmt.Errorf("duplicate permission for user %s", permission.UserID)
			}
			permissions[permission.UserID] = struct{}{}
		}
	}
	return nil
}
