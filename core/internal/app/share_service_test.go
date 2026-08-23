package app

import (
	"context"
	"errors"
	"testing"

	"davdeck.dev/davdeck/core/internal/domain"
)

type memoryShares struct{ shares map[domain.ID]domain.Share }

func newMemoryShares() *memoryShares { return &memoryShares{shares: make(map[domain.ID]domain.Share)} }
func (r *memoryShares) List(context.Context) ([]domain.Share, error) {
	result := make([]domain.Share, 0, len(r.shares))
	for _, share := range r.shares {
		result = append(result, share)
	}
	return result, nil
}
func (r *memoryShares) Get(_ context.Context, id domain.ID) (domain.Share, error) {
	share, ok := r.shares[id]
	if !ok {
		return domain.Share{}, ErrShareNotFound
	}
	return share, nil
}
func (r *memoryShares) Create(_ context.Context, share domain.Share) error {
	for _, existing := range r.shares {
		if existing.Slug == share.Slug {
			return ErrShareAlreadyExists
		}
	}
	r.shares[share.ID] = share
	return nil
}
func (r *memoryShares) Update(_ context.Context, share domain.Share) error {
	if _, ok := r.shares[share.ID]; !ok {
		return ErrShareNotFound
	}
	for id, existing := range r.shares {
		if id != share.ID && existing.Slug == share.Slug {
			return ErrShareAlreadyExists
		}
	}
	r.shares[share.ID] = share
	return nil
}
func (r *memoryShares) Delete(_ context.Context, id domain.ID) error {
	if _, ok := r.shares[id]; !ok {
		return ErrShareNotFound
	}
	delete(r.shares, id)
	return nil
}

type fakeSharePaths struct {
	err       error
	validated []string
}

func (p *fakeSharePaths) ValidateSharePath(path string) error {
	p.validated = append(p.validated, path)
	return p.err
}

func TestShareServiceLifecycle(t *testing.T) {
	repository, paths := newMemoryShares(), &fakeSharePaths{}
	service := NewShareService(repository, paths, fixedID{}, fixedClock{})
	share, err := service.Create(context.Background(), "Documents", "documents", "/srv/documents")
	if err != nil {
		t.Fatal(err)
	}
	if !share.Enabled || len(paths.validated) != 1 {
		t.Fatalf("share = %#v, paths = %#v", share, paths.validated)
	}
	name, slug, path, enabled := "Team Documents", "team-documents", "/srv/team", false
	updated, err := service.Update(context.Background(), share.ID, ShareUpdate{Name: &name, Slug: &slug, Path: &path, Enabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != name || updated.Slug != slug || updated.Path != path || updated.Enabled {
		t.Fatalf("updated = %#v", updated)
	}
	if err := service.Delete(context.Background(), share.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(context.Background(), share.ID); !hasCode(err, CodeShareNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestShareServiceValidation(t *testing.T) {
	paths := &fakeSharePaths{}
	service := NewShareService(newMemoryShares(), paths, fixedID{}, fixedClock{})
	for _, testCase := range []struct {
		name, slug, path string
		code             ErrorCode
	}{
		{" Documents ", "documents", "/srv/documents", CodeInvalidShareName},
		{"Documents", "../documents", "/srv/documents", CodeInvalidShareSlug},
		{"Documents", "documents", "relative", CodeInvalidSharePath},
	} {
		if _, err := service.Create(context.Background(), testCase.name, testCase.slug, testCase.path); !hasCode(err, testCase.code) {
			t.Errorf("Create(%q, %q, %q) error = %v", testCase.name, testCase.slug, testCase.path, err)
		}
	}
	paths.err = ErrSharePathNotFound
	if _, err := service.Create(context.Background(), "Documents", "documents", "/missing"); !hasCode(err, CodeSharePathNotFound) {
		t.Fatalf("missing path error = %v", err)
	}
	paths.err = ErrSharePathUnreadable
	if _, err := service.Create(context.Background(), "Documents", "documents", "/unreadable"); !hasCode(err, CodeSharePathUnreadable) {
		t.Fatalf("unreadable error = %v", err)
	}
	paths.err = ErrSharePathUnwritable
	if _, err := service.Create(context.Background(), "Documents", "documents", "/unwritable"); !hasCode(err, CodeSharePathUnwritable) {
		t.Fatalf("unwritable error = %v", err)
	}
}

func TestShareServiceDuplicateSlug(t *testing.T) {
	repository := newMemoryShares()
	paths := &fakeSharePaths{}
	service := NewShareService(repository, paths, fixedID{}, fixedClock{})
	if _, err := service.Create(context.Background(), "Documents", "documents", "/srv/documents"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), "Other", "documents", "/srv/other"); !hasCode(err, CodeShareAlreadyExists) {
		t.Fatalf("error = %v", err)
	}
	if !errors.Is(mapShareError(ErrShareAlreadyExists), ErrShareAlreadyExists) {
		t.Fatal("cause was not retained")
	}
}
