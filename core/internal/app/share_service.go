package app

import (
	"context"
	"errors"
	"fmt"

	"davdeck.dev/davdeck/core/internal/domain"
)

var (
	ErrShareNotFound       = errors.New("share not found")
	ErrShareAlreadyExists  = errors.New("share already exists")
	ErrSharePathNotFound   = errors.New("share path not found")
	ErrSharePathUnreadable = errors.New("share path is not readable")
	ErrSharePathUnwritable = errors.New("share path is not writable")
)

type ShareRepository interface {
	List(context.Context) ([]domain.Share, error)
	Get(context.Context, domain.ID) (domain.Share, error)
	Create(context.Context, domain.Share) error
	Update(context.Context, domain.Share) error
	Delete(context.Context, domain.ID) error
}

type SharePathValidator interface{ ValidateSharePath(string) error }

type ShareUpdate struct {
	Name    *string
	Slug    *string
	Path    *string
	Enabled *bool
}

type ShareService struct {
	repository ShareRepository
	paths      SharePathValidator
	ids        IDGenerator
	clock      Clock
}

func NewShareService(repository ShareRepository, paths SharePathValidator, ids IDGenerator, clock Clock) *ShareService {
	return &ShareService{repository: repository, paths: paths, ids: ids, clock: clock}
}

func (s *ShareService) List(ctx context.Context) ([]domain.Share, error) {
	shares, err := s.repository.List(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	return shares, nil
}

func (s *ShareService) Get(ctx context.Context, id domain.ID) (domain.Share, error) {
	share, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Share{}, mapShareError(err)
	}
	return share, nil
}

func (s *ShareService) Create(ctx context.Context, name, slug, path string) (domain.Share, error) {
	id, err := s.ids.NewID()
	if err != nil {
		return domain.Share{}, databaseError(fmt.Errorf("generate share id: %w", err))
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return domain.Share{}, databaseError(err)
	}
	share := domain.Share{ID: id, Name: name, Slug: slug, Path: path, Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	if err := validateShare(share); err != nil {
		return domain.Share{}, err
	}
	if err := s.paths.ValidateSharePath(path); err != nil {
		return domain.Share{}, mapShareError(err)
	}
	if err := s.repository.Create(ctx, share); err != nil {
		return domain.Share{}, mapShareError(err)
	}
	return share, nil
}

func (s *ShareService) Update(ctx context.Context, id domain.ID, update ShareUpdate) (domain.Share, error) {
	share, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Share{}, mapShareError(err)
	}
	if update.Name != nil {
		share.Name = *update.Name
	}
	if update.Slug != nil {
		share.Slug = *update.Slug
	}
	if update.Path != nil {
		share.Path = *update.Path
	}
	if update.Enabled != nil {
		share.Enabled = *update.Enabled
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return domain.Share{}, databaseError(err)
	}
	share.UpdatedAt = stamp
	if err := validateShare(share); err != nil {
		return domain.Share{}, err
	}
	if update.Path != nil {
		if err := s.paths.ValidateSharePath(share.Path); err != nil {
			return domain.Share{}, mapShareError(err)
		}
	}
	if err := s.repository.Update(ctx, share); err != nil {
		return domain.Share{}, mapShareError(err)
	}
	return share, nil
}

func (s *ShareService) Delete(ctx context.Context, id domain.ID) error {
	return mapShareError(s.repository.Delete(ctx, id))
}

func validateShare(share domain.Share) error {
	if err := share.Validate(); err != nil {
		var validation *domain.ValidationError
		if errors.As(err, &validation) {
			switch validation.Code {
			case domain.CodeInvalidShareName:
				return &Error{Code: CodeInvalidShareName, Message: "Share name is invalid", Cause: err}
			case domain.CodeInvalidShareSlug:
				return &Error{Code: CodeInvalidShareSlug, Message: "Share slug is invalid", Cause: err}
			case domain.CodeInvalidSharePath:
				return &Error{Code: CodeInvalidSharePath, Message: "Share path is invalid", Cause: err}
			}
		}
		return &Error{Code: CodeInvalidSharePath, Message: "Share is invalid", Cause: err}
	}
	return nil
}

func mapShareError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrShareNotFound):
		return &Error{Code: CodeShareNotFound, Message: "Share was not found", Cause: err}
	case errors.Is(err, ErrShareAlreadyExists):
		return &Error{Code: CodeShareAlreadyExists, Message: "Share slug already exists", Cause: err}
	case errors.Is(err, ErrSharePathNotFound):
		return &Error{Code: CodeSharePathNotFound, Message: "Share directory does not exist", Cause: err}
	case errors.Is(err, ErrSharePathUnreadable):
		return &Error{Code: CodeSharePathUnreadable, Message: "Share directory is not readable", Cause: err}
	case errors.Is(err, ErrSharePathUnwritable):
		return &Error{Code: CodeSharePathUnwritable, Message: "Share directory is not writable", Cause: err}
	default:
		return databaseError(err)
	}
}
