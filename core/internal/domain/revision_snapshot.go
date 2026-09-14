package domain

import (
	"crypto/sha256"
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
	DNSProviders   []DNSProviderCredential       `json:"dns_providers,omitempty"`
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
	dnsProviders := append([]DNSProviderCredential(nil), input.DNSProviderCredentials...)
	for index := range dnsProviders {
		dnsProviders[index].AllowedZones = append([]string(nil), dnsProviders[index].AllowedZones...)
		sort.Strings(dnsProviders[index].AllowedZones)
	}
	sort.Slice(dnsProviders, func(i, j int) bool {
		if dnsProviders[i].Name == dnsProviders[j].Name {
			return dnsProviders[i].ID < dnsProviders[j].ID
		}
		return dnsProviders[i].Name < dnsProviders[j].Name
	})
	return ConfigRevisionSnapshot{
		Version: ConfigRevisionSnapshotVersion, ServerSettings: input.ServerSettings,
		TLSProfile: tlsProfile, DNSProviders: dnsProviders, Users: users, Shares: shares,
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
	dnsProviders := append([]DNSProviderCredential(nil), s.DNSProviders...)
	return RuntimeConfigInput{
		ServerSettings: s.ServerSettings, TLSProfile: tlsProfile,
		DNSProviderCredentials: dnsProviders, Users: users, Shares: shares,
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

// HashConfigRevisionState returns a stable identity for the semantic desired
// state represented by input. Persistence metadata such as created_at and
// updated_at is deliberately excluded; those fields are useful for display
// and rollback fidelity but must not turn an equivalent state into a new
// revision.
func HashConfigRevisionState(input RuntimeConfigInput) (string, error) {
	snapshot := NewConfigRevisionSnapshot(input)
	if err := snapshot.Validate(); err != nil {
		return "", err
	}
	body, err := json.Marshal(revisionStateIdentityFromSnapshot(snapshot))
	if err != nil {
		return "", fmt.Errorf("encode revision state identity: %w", err)
	}
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum[:]), nil
}

// HashConfigRevisionStateSnapshot computes the semantic identity of a stored
// private revision snapshot. It is used to match revisions created before the
// state_hash column was introduced.
func HashConfigRevisionStateSnapshot(body []byte) (string, error) {
	input, err := ParseConfigRevisionSnapshot(body)
	if err != nil {
		return "", err
	}
	return HashConfigRevisionState(input)
}

type revisionStateIdentity struct {
	Version        int                         `json:"version"`
	ServerSettings revisionStateServerSettings `json:"server_settings"`
	TLSProfile     *revisionStateTLSProfile    `json:"tls_profile,omitempty"`
	DNSProviders   []revisionStateDNSProvider  `json:"dns_providers,omitempty"`
	Users          []revisionStateUser         `json:"users"`
	Shares         []revisionStateShare        `json:"shares"`
}

type revisionStateServerSettings struct {
	ID             ID          `json:"id"`
	PublicBasePath string      `json:"public_base_path"`
	HTTPPort       int         `json:"http_port"`
	HTTPSPort      int         `json:"https_port"`
	RuntimeMode    RuntimeMode `json:"runtime_mode"`
}

type revisionStateTLSProfile struct {
	ID              ID           `json:"id"`
	Mode            TLSMode      `json:"mode"`
	Hostname        string       `json:"hostname"`
	Challenge       TLSChallenge `json:"challenge"`
	DNSProviderID   *ID          `json:"dns_provider_id,omitempty"`
	CertificatePath string       `json:"certificate_path,omitempty"`
	PrivateKeyPath  string       `json:"private_key_path,omitempty"`
}

type revisionStateDNSProvider struct {
	ID           ID              `json:"id"`
	Name         string          `json:"name"`
	Provider     DNSProviderType `json:"provider"`
	AllowedZones []string        `json:"allowed_zones,omitempty"`
}

type revisionStateUser struct {
	ID                 ID     `json:"id"`
	Username           string `json:"username"`
	UsernameNormalized string `json:"username_normalized"`
	PasswordHash       string `json:"password_hash"`
	Enabled            bool   `json:"enabled"`
}

type revisionStateShare struct {
	ID          ID                        `json:"id"`
	Name        string                    `json:"name"`
	Slug        string                    `json:"slug"`
	Path        string                    `json:"path"`
	Enabled     bool                      `json:"enabled"`
	Permissions []revisionStatePermission `json:"permissions"`
}

type revisionStatePermission struct {
	ShareID    ID         `json:"share_id"`
	UserID     ID         `json:"user_id"`
	Permission Permission `json:"permission"`
}

func revisionStateIdentityFromSnapshot(snapshot ConfigRevisionSnapshot) revisionStateIdentity {
	identity := revisionStateIdentity{
		Version: snapshot.Version,
		ServerSettings: revisionStateServerSettings{
			ID: snapshot.ServerSettings.ID, PublicBasePath: snapshot.ServerSettings.PublicBasePath,
			HTTPPort: snapshot.ServerSettings.HTTPPort, HTTPSPort: snapshot.ServerSettings.HTTPSPort,
			RuntimeMode: snapshot.ServerSettings.RuntimeMode,
		},
		DNSProviders: make([]revisionStateDNSProvider, 0, len(snapshot.DNSProviders)),
		Users:        make([]revisionStateUser, 0, len(snapshot.Users)),
		Shares:       make([]revisionStateShare, 0, len(snapshot.Shares)),
	}
	if snapshot.TLSProfile != nil {
		profile := snapshot.TLSProfile
		challenge := profile.Challenge
		if challenge == "" {
			challenge = TLSChallengeAuto
		}
		var providerID *ID
		if profile.DNSProviderID != nil {
			value := *profile.DNSProviderID
			providerID = &value
		}
		identity.TLSProfile = &revisionStateTLSProfile{
			ID: profile.ID, Mode: profile.Mode, Hostname: profile.Hostname, Challenge: challenge,
			DNSProviderID: providerID, CertificatePath: profile.CertificatePath,
			PrivateKeyPath: profile.PrivateKeyPath,
		}
	}
	for _, provider := range snapshot.DNSProviders {
		identity.DNSProviders = append(identity.DNSProviders, revisionStateDNSProvider{
			ID: provider.ID, Name: provider.Name, Provider: provider.Provider,
			AllowedZones: append([]string(nil), provider.AllowedZones...),
		})
	}
	for _, user := range snapshot.Users {
		identity.Users = append(identity.Users, revisionStateUser{
			ID: user.ID, Username: user.Username, UsernameNormalized: user.UsernameNormalized,
			PasswordHash: user.PasswordHash, Enabled: user.Enabled,
		})
	}
	for _, item := range snapshot.Shares {
		share := item.Share
		permissions := make([]revisionStatePermission, 0, len(item.Permissions))
		for _, permission := range item.Permissions {
			// NONE and an absent row are semantically equivalent in the MVP
			// permission model.
			if permission.Permission == PermissionNone {
				continue
			}
			permissions = append(permissions, revisionStatePermission{
				ShareID: permission.ShareID, UserID: permission.UserID, Permission: permission.Permission,
			})
		}
		identity.Shares = append(identity.Shares, revisionStateShare{
			ID: share.ID, Name: share.Name, Slug: share.Slug, Path: share.Path,
			Enabled: share.Enabled, Permissions: permissions,
		})
	}
	return identity
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
	providers := make(map[ID]struct{}, len(input.DNSProviderCredentials))
	providerNames := make(map[string]struct{}, len(input.DNSProviderCredentials))
	for _, provider := range input.DNSProviderCredentials {
		if err := provider.Validate(); err != nil {
			return fmt.Errorf("DNS provider %s: %w", provider.ID, err)
		}
		if _, exists := providers[provider.ID]; exists {
			return fmt.Errorf("duplicate DNS provider id %s", provider.ID)
		}
		if _, exists := providerNames[provider.Name]; exists {
			return fmt.Errorf("duplicate DNS provider name %s", provider.Name)
		}
		providers[provider.ID] = struct{}{}
		providerNames[provider.Name] = struct{}{}
	}
	if input.TLSProfile != nil && input.TLSProfile.Challenge == TLSChallengeDNS {
		if input.TLSProfile.DNSProviderID == nil {
			return fmt.Errorf("DNS TLS profile has no provider")
		}
		if _, exists := providers[*input.TLSProfile.DNSProviderID]; !exists {
			return fmt.Errorf("DNS TLS profile references unknown provider %s", *input.TLSProfile.DNSProviderID)
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
