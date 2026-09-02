package services

import (
	"testing"

	"github.com/google/uuid"

	"github.com/d2cTool/goprofile/internal/domain"
)

func TestPublicURL(t *testing.T) {
	t.Parallel()
	svc := NewAvatarService(nil, nil, nil, "http://localhost:8080/", 0)
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if got := svc.PublicURL(id, domain.SizeOriginal); got != "http://localhost:8080/api/v1/avatars/"+id.String() {
		t.Fatal(got)
	}
	if got := svc.PublicURL(id, domain.Size300); got != "http://localhost:8080/api/v1/avatars/"+id.String()+"?size=300x300" {
		t.Fatal(got)
	}
}
