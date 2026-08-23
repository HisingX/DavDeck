package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

type memoryServerSettings struct{ value domain.ServerSettings }

func (r *memoryServerSettings) Get(context.Context) (domain.ServerSettings, error) {
	return r.value, nil
}
func (r *memoryServerSettings) UpdatePorts(_ context.Context, httpPort, httpsPort int, stamp domain.Timestamp) (domain.ServerSettings, error) {
	r.value.HTTPPort, r.value.HTTPSPort, r.value.UpdatedAt = httpPort, httpsPort, stamp
	return r.value, nil
}

type testPortChecker struct{ unavailable int }

func (c testPortChecker) Check(_ context.Context, port int) error {
	if port == c.unavailable {
		return errors.New("occupied")
	}
	return nil
}

func TestServerSettingsServiceRejectsUnavailablePortWithoutPersisting(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC))
	repository := &memoryServerSettings{value: domain.ServerSettings{ID: "11111111-1111-4111-8111-111111111111", PublicBasePath: "/dav", HTTPPort: 8080, HTTPSPort: 8443, RuntimeMode: domain.RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp}}
	service := NewServerSettingsService(repository, testPortChecker{unavailable: 9080}, fixedClock{})
	_, err := service.UpdatePorts(context.Background(), 9080, 9443)
	var applicationError *Error
	if !errors.As(err, &applicationError) || applicationError.Code != CodeServerPortUnavailable {
		t.Fatalf("error = %#v", err)
	}
	if repository.value.HTTPPort != 8080 || repository.value.HTTPSPort != 8443 {
		t.Fatalf("ports persisted despite failed preflight: %#v", repository.value)
	}
}

func TestServerSettingsServiceUpdatesValidatedPorts(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC))
	repository := &memoryServerSettings{value: domain.ServerSettings{ID: "11111111-1111-4111-8111-111111111111", PublicBasePath: "/dav", HTTPPort: 8080, HTTPSPort: 8443, RuntimeMode: domain.RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp}}
	service := NewServerSettingsService(repository, testPortChecker{}, fixedClock{})
	settings, err := service.UpdatePorts(context.Background(), 9080, 9443)
	if err != nil || settings.HTTPPort != 9080 || settings.HTTPSPort != 9443 {
		t.Fatalf("settings = %#v, err = %v", settings, err)
	}
}
