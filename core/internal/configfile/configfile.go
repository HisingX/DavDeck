// Package configfile owns DavDeck's versioned, no-secret YAML interchange format.
package configfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
	"go.yaml.in/yaml/v3"
)

const (
	Version          = 1
	MaximumSizeBytes = 1 << 20
	maximumNodes     = 10000
	maximumDepth     = 64
)

type Document struct {
	Version int     `yaml:"version"`
	Server  *Server `yaml:"server,omitempty"`
	TLS     *TLS    `yaml:"tls,omitempty"`
	Users   []User  `yaml:"users,omitempty"`
	Shares  []Share `yaml:"shares,omitempty"`
}

type Server struct {
	PublicBasePath string `yaml:"public_base_path"`
	HTTPPort       int    `yaml:"http_port"`
	HTTPSPort      int    `yaml:"https_port"`
	RuntimeMode    string `yaml:"runtime_mode"`
}

type TLS struct {
	Mode            string `yaml:"mode"`
	Hostname        string `yaml:"hostname"`
	CertificatePath string `yaml:"certificate_path,omitempty"`
	PrivateKeyPath  string `yaml:"private_key_path,omitempty"`
}

type User struct {
	Username string `yaml:"username"`
	Enabled  *bool  `yaml:"enabled"`
}

type Share struct {
	Name        string      `yaml:"name"`
	Slug        string      `yaml:"slug"`
	Path        string      `yaml:"path"`
	Enabled     *bool       `yaml:"enabled"`
	Permissions Permissions `yaml:"permissions,omitempty"`
}

type Permissions map[string]string

func (p Permissions) MarshalYAML() (any, error) {
	keys := make([]string, 0, len(p))
	for key := range p {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return domain.NormalizeUsername(keys[i]) < domain.NormalizeUsername(keys[j]) })
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, key := range keys {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: p[key]},
		)
	}
	return node, nil
}

type ErrorCode string

const (
	CodeInvalid     ErrorCode = "CONFIG_IMPORT_INVALID"
	CodeUnsupported ErrorCode = "CONFIG_VERSION_UNSUPPORTED"
)

type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return string(e.Code) + ": " + e.Message
	}
	return string(e.Code) + ": " + e.Message + ": " + e.Cause.Error()
}
func (e *Error) Unwrap() error { return e.Cause }

func Parse(body []byte) (Document, error) {
	if len(body) == 0 || len(body) > MaximumSizeBytes {
		return Document{}, invalid("Configuration must contain between 1 byte and 1 MiB", nil)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(body, &root); err != nil {
		return Document{}, invalid("Configuration is not valid YAML", err)
	}
	count := 0
	if err := inspectNode(&root, 0, &count); err != nil {
		return Document{}, invalid("Configuration uses an unsupported YAML feature", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	var document Document
	if err := decoder.Decode(&document); err != nil {
		return Document{}, invalid("Configuration contains unknown or invalid fields", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Document{}, invalid("Configuration must contain exactly one YAML document", err)
	}
	if document.Version != Version {
		return Document{}, &Error{Code: CodeUnsupported, Message: "Only configuration version 1 is supported"}
	}
	if err := document.Validate(); err != nil {
		return Document{}, invalid("Configuration validation failed", err)
	}
	return document, nil
}

func (d Document) Validate() error {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	const placeholderID = domain.ID("00000000-0000-4000-8000-000000000001")
	if d.Server != nil {
		settings := domain.ServerSettings{ID: placeholderID, PublicBasePath: d.Server.PublicBasePath, HTTPPort: d.Server.HTTPPort, HTTPSPort: d.Server.HTTPSPort, RuntimeMode: domain.RuntimeMode(d.Server.RuntimeMode), CreatedAt: stamp, UpdatedAt: stamp}
		if err := settings.Validate(); err != nil {
			return fmt.Errorf("server: %w", err)
		}
	}
	if d.TLS != nil {
		profile := domain.TLSProfile{ID: placeholderID, Mode: domain.TLSMode(d.TLS.Mode), Hostname: d.TLS.Hostname, CertificatePath: d.TLS.CertificatePath, PrivateKeyPath: d.TLS.PrivateKeyPath, CreatedAt: stamp, UpdatedAt: stamp}
		if err := profile.Validate(); err != nil {
			return fmt.Errorf("tls: %w", err)
		}
	}
	users := make(map[string]struct{}, len(d.Users))
	for index, value := range d.Users {
		if value.Enabled == nil {
			return fmt.Errorf("users[%d].enabled is required", index)
		}
		normalized := domain.NormalizeUsername(value.Username)
		user := domain.User{ID: placeholderID, Username: value.Username, UsernameNormalized: normalized, PasswordHash: "not-exported", Enabled: *value.Enabled, CreatedAt: stamp, UpdatedAt: stamp}
		if err := user.Validate(); err != nil {
			return fmt.Errorf("users[%d]: %w", index, err)
		}
		if _, exists := users[normalized]; exists {
			return fmt.Errorf("users[%d]: duplicate normalized username", index)
		}
		users[normalized] = struct{}{}
	}
	shareSlugs := make(map[string]struct{}, len(d.Shares))
	for index, value := range d.Shares {
		if value.Enabled == nil {
			return fmt.Errorf("shares[%d].enabled is required", index)
		}
		share := domain.Share{ID: placeholderID, Name: value.Name, Slug: value.Slug, Path: value.Path, Enabled: *value.Enabled, CreatedAt: stamp, UpdatedAt: stamp}
		if err := share.Validate(); err != nil {
			return fmt.Errorf("shares[%d]: %w", index, err)
		}
		if _, exists := shareSlugs[value.Slug]; exists {
			return fmt.Errorf("shares[%d]: duplicate slug", index)
		}
		shareSlugs[value.Slug] = struct{}{}
		permissionUsers := make(map[string]struct{}, len(value.Permissions))
		for username, permission := range value.Permissions {
			normalized := domain.NormalizeUsername(username)
			permissionUser := domain.User{ID: placeholderID, Username: username, UsernameNormalized: normalized, PasswordHash: "not-exported", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
			if err := permissionUser.Validate(); err != nil {
				return fmt.Errorf("shares[%d].permissions contains an invalid username", index)
			}
			if _, exists := permissionUsers[normalized]; exists {
				return fmt.Errorf("shares[%d].permissions contains duplicate normalized usernames", index)
			}
			permissionUsers[normalized] = struct{}{}
			if _, ok := ParsePermission(permission); !ok {
				return fmt.Errorf("shares[%d].permissions contains an invalid permission", index)
			}
		}
	}
	return nil
}

func ParsePermission(value string) (domain.Permission, bool) {
	switch value {
	case "none":
		return domain.PermissionNone, true
	case "read":
		return domain.PermissionRead, true
	case "read_write":
		return domain.PermissionReadWrite, true
	default:
		return "", false
	}
}

func PermissionValue(value domain.Permission) (string, error) {
	switch value {
	case domain.PermissionNone:
		return "none", nil
	case domain.PermissionRead:
		return "read", nil
	case domain.PermissionReadWrite:
		return "read_write", nil
	default:
		return "", fmt.Errorf("invalid permission")
	}
}

func inspectNode(node *yaml.Node, depth int, count *int) error {
	(*count)++
	if *count > maximumNodes || depth > maximumDepth {
		return errors.New("YAML structure exceeds safety limits")
	}
	if node.Kind == yaml.AliasNode || node.Alias != nil {
		return errors.New("YAML aliases are not supported")
	}
	allowedTags := map[string]bool{"": true, "!!map": true, "!!seq": true, "!!str": true, "!!int": true, "!!bool": true, "!!null": true}
	if !allowedTags[node.ShortTag()] {
		return errors.New("custom YAML tags are not supported")
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			if index+1 >= len(node.Content) || node.Content[index].Kind != yaml.ScalarNode {
				return errors.New("mapping keys must be scalar values")
			}
			key := node.Content[index].Value
			if key == "<<" {
				return errors.New("YAML merge keys are not supported")
			}
			if _, exists := seen[key]; exists {
				return errors.New("duplicate YAML mapping key")
			}
			seen[key] = struct{}{}
		}
	}
	for _, child := range node.Content {
		if err := inspectNode(child, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

func invalid(message string, cause error) error {
	return &Error{Code: CodeInvalid, Message: message, Cause: cause}
}

func boolPointer(value bool) *bool { return &value }

func normalizeDocument(document *Document) {
	sort.Slice(document.Users, func(i, j int) bool {
		return domain.NormalizeUsername(document.Users[i].Username) < domain.NormalizeUsername(document.Users[j].Username)
	})
	sort.Slice(document.Shares, func(i, j int) bool { return document.Shares[i].Slug < document.Shares[j].Slug })
}

func Export(input domain.RuntimeConfigInput) ([]byte, error) {
	document := Document{Version: Version, Server: &Server{PublicBasePath: input.ServerSettings.PublicBasePath, HTTPPort: input.ServerSettings.HTTPPort, HTTPSPort: input.ServerSettings.HTTPSPort, RuntimeMode: string(input.ServerSettings.RuntimeMode)}}
	if input.TLSProfile != nil {
		document.TLS = &TLS{Mode: string(input.TLSProfile.Mode), Hostname: input.TLSProfile.Hostname, CertificatePath: input.TLSProfile.CertificatePath, PrivateKeyPath: input.TLSProfile.PrivateKeyPath}
	}
	userByID := make(map[domain.ID]string, len(input.Users))
	for _, user := range input.Users {
		userByID[user.ID] = user.Username
		document.Users = append(document.Users, User{Username: user.Username, Enabled: boolPointer(user.Enabled)})
	}
	for _, item := range input.Shares {
		share := Share{Name: item.Share.Name, Slug: item.Share.Slug, Path: item.Share.Path, Enabled: boolPointer(item.Share.Enabled), Permissions: make(Permissions)}
		for _, entry := range item.Permissions {
			username, exists := userByID[entry.UserID]
			if !exists {
				return nil, fmt.Errorf("permission references unknown user")
			}
			value, err := PermissionValue(entry.Permission)
			if err != nil {
				return nil, err
			}
			share.Permissions[username] = value
		}
		document.Shares = append(document.Shares, share)
	}
	normalizeDocument(&document)
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("encode YAML configuration: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close YAML encoder: %w", err)
	}
	return output.Bytes(), nil
}

func ImportedUsernames(document Document) []string {
	result := make([]string, 0, len(document.Users))
	for _, user := range document.Users {
		result = append(result, user.Username)
	}
	sort.Slice(result, func(i, j int) bool { return domain.NormalizeUsername(result[i]) < domain.NormalizeUsername(result[j]) })
	return result
}
