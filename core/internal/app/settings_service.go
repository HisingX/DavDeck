package app

import (
	"context"
	"errors"
	"fmt"

	"davdeck.dev/davdeck/core/internal/domain"
)

var ErrServerSettingsNotFound = errors.New("server settings not found")

type ServerSettingsRepository interface {
	Get(context.Context) (domain.ServerSettings, error)
	UpdatePorts(context.Context, int, int, domain.Timestamp) (domain.ServerSettings, error)
}

type ListenerPortChecker interface {
	Check(context.Context, int) error
}

type ServerSettingsService struct {
	repository ServerSettingsRepository
	ports      ListenerPortChecker
	clock      Clock
}

func NewServerSettingsService(repository ServerSettingsRepository, ports ListenerPortChecker, clock Clock) *ServerSettingsService {
	return &ServerSettingsService{repository: repository, ports: ports, clock: clock}
}

func (s *ServerSettingsService) Get(ctx context.Context) (domain.ServerSettings, error) {
	settings, err := s.repository.Get(ctx)
	if err != nil {
		return domain.ServerSettings{}, mapServerSettingsError(err)
	}
	return settings, nil
}

func (s *ServerSettingsService) UpdatePorts(ctx context.Context, httpPort, httpsPort int) (domain.ServerSettings, error) {
	current, err := s.Get(ctx)
	if err != nil {
		return domain.ServerSettings{}, err
	}
	candidate := current
	candidate.HTTPPort, candidate.HTTPSPort = httpPort, httpsPort
	if err := candidate.Validate(); err != nil {
		return domain.ServerSettings{}, &Error{Code: CodeInvalidServerPorts, Message: "Server ports are invalid", Cause: err}
	}
	for _, port := range []int{httpPort, httpsPort} {
		if port == current.HTTPPort || port == current.HTTPSPort {
			continue
		}
		if err := s.ports.Check(ctx, port); err != nil {
			return domain.ServerSettings{}, &Error{Code: CodeServerPortUnavailable, Message: fmt.Sprintf("Port %d is unavailable", port), Cause: err}
		}
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return domain.ServerSettings{}, databaseError(err)
	}
	settings, err := s.repository.UpdatePorts(ctx, httpPort, httpsPort, stamp)
	if err != nil {
		return domain.ServerSettings{}, mapServerSettingsError(err)
	}
	return settings, nil
}

func mapServerSettingsError(err error) error {
	if errors.Is(err, ErrServerSettingsNotFound) {
		return &Error{Code: CodeServerSettingsNotFound, Message: "Server settings were not found", Cause: err}
	}
	return databaseError(err)
}
