package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"

	"davdeck.dev/davdeck/core/internal/configfile"
	"davdeck.dev/davdeck/core/internal/domain"
)

type ConfigImportSeed struct {
	Document   configfile.Document
	UserIDs    map[string]domain.ID
	UserHashes map[string]string
	ShareIDs   map[string]domain.ID
	TLSID      domain.ID
	Timestamp  domain.Timestamp
}

type ConfigImportResult struct {
	UsersCreated          int      `json:"users_created"`
	UsersUpdated          int      `json:"users_updated"`
	SharesCreated         int      `json:"shares_created"`
	SharesUpdated         int      `json:"shares_updated"`
	PermissionsUpserted   int      `json:"permissions_upserted"`
	TLSUpdated            bool     `json:"tls_updated"`
	ServerUpdated         bool     `json:"server_updated"`
	PasswordResetRequired []string `json:"password_reset_required"`
	PendingApply          bool     `json:"pending_apply"`
}

type ConfigImportRepository interface {
	Import(context.Context, ConfigImportSeed) (ConfigImportResult, error)
}

type ConfigService struct {
	snapshots  SnapshotProvider
	imports    ConfigImportRepository
	paths      SharePathValidator
	hasher     PasswordHasher
	ids        IDGenerator
	clock      Clock
	randomRead func([]byte) (int, error)
}

func NewConfigService(snapshots SnapshotProvider, imports ConfigImportRepository, paths SharePathValidator, hasher PasswordHasher, ids IDGenerator, clock Clock) *ConfigService {
	return &ConfigService{snapshots: snapshots, imports: imports, paths: paths, hasher: hasher, ids: ids, clock: clock, randomRead: rand.Read}
}

func (s *ConfigService) Export(ctx context.Context) ([]byte, error) {
	snapshot, err := s.snapshots.Snapshot(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	body, err := configfile.Export(snapshot)
	if err != nil {
		return nil, &Error{Code: CodeConfigExportFailed, Message: "Configuration could not be exported", Cause: err}
	}
	return body, nil
}

func (s *ConfigService) Import(ctx context.Context, body []byte) (ConfigImportResult, error) {
	document, err := configfile.Parse(body)
	if err != nil {
		return ConfigImportResult{}, mapConfigFileError(err)
	}
	for _, share := range document.Shares {
		if share.Enabled != nil && *share.Enabled {
			if err := s.paths.ValidateSharePath(share.Path); err != nil {
				return ConfigImportResult{}, &Error{Code: CodeConfigImportInvalid, Message: "An enabled share path failed validation", Cause: err}
			}
		}
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return ConfigImportResult{}, databaseError(err)
	}
	seed := ConfigImportSeed{Document: document, UserIDs: make(map[string]domain.ID), UserHashes: make(map[string]string), ShareIDs: make(map[string]domain.ID), Timestamp: stamp}
	placeholderHash := ""
	if len(document.Users) > 0 {
		randomPassword := make([]byte, 32)
		count, err := s.randomRead(randomPassword)
		if err != nil {
			return ConfigImportResult{}, databaseError(fmt.Errorf("generate imported user credential: %w", err))
		}
		if count != len(randomPassword) {
			return ConfigImportResult{}, databaseError(errors.New("generate imported user credential: short random read"))
		}
		placeholderHash, err = s.hasher.Hash(base64.RawURLEncoding.EncodeToString(randomPassword))
		if err != nil {
			return ConfigImportResult{}, &Error{Code: CodeConfigImportInvalid, Message: "Imported user placeholder credential could not be secured", Cause: err}
		}
	}
	for _, user := range document.Users {
		normalized := domain.NormalizeUsername(user.Username)
		id, err := s.ids.NewID()
		if err != nil {
			return ConfigImportResult{}, databaseError(err)
		}
		seed.UserIDs[normalized], seed.UserHashes[normalized] = id, placeholderHash
	}
	for _, share := range document.Shares {
		id, err := s.ids.NewID()
		if err != nil {
			return ConfigImportResult{}, databaseError(err)
		}
		seed.ShareIDs[share.Slug] = id
	}
	if document.TLS != nil {
		seed.TLSID, err = s.ids.NewID()
		if err != nil {
			return ConfigImportResult{}, databaseError(err)
		}
	}
	result, err := s.imports.Import(ctx, seed)
	if err != nil {
		if errors.Is(err, ErrConfigImportConflict) {
			return ConfigImportResult{}, &Error{Code: CodeConfigImportInvalid, Message: "Configuration references missing or conflicting resources", Cause: err}
		}
		return ConfigImportResult{}, databaseError(err)
	}
	sort.Slice(result.PasswordResetRequired, func(i, j int) bool {
		return domain.NormalizeUsername(result.PasswordResetRequired[i]) < domain.NormalizeUsername(result.PasswordResetRequired[j])
	})
	result.PendingApply = true
	return result, nil
}

var ErrConfigImportConflict = errors.New("config import conflict")

func mapConfigFileError(err error) error {
	var configError *configfile.Error
	if errors.As(err, &configError) && configError.Code == configfile.CodeUnsupported {
		return &Error{Code: CodeConfigVersionUnsupported, Message: "Configuration version is unsupported", Cause: err}
	}
	return &Error{Code: CodeConfigImportInvalid, Message: "Configuration is invalid", Cause: err}
}
