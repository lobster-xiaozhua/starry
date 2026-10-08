package auth

import (
	"testing"

	"github.com/google/uuid"

	"starry/backend/internal/model"
)

func TestBuildAuthAudit_KnownUser(t *testing.T) {
	u := &model.User{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Username: "alice"}
	e := buildAuthAudit("auth.login.success", "success", u, "10.0.0.1", "")
	if e.ActorID != u.ID.String() {
		t.Fatalf("ActorID = %q, want %q", e.ActorID, u.ID.String())
	}
	if e.ActorName != "alice" {
		t.Fatalf("ActorName = %q, want alice", e.ActorName)
	}
	if e.TargetID != u.ID.String() || e.TargetName != "alice" {
		t.Fatalf("target fields not mirrored: %+v", e)
	}
	if e.Outcome != "success" || e.Action != "auth.login.success" || e.IP != "10.0.0.1" {
		t.Fatalf("core fields wrong: %+v", e)
	}
}

func TestBuildAuthAudit_UnknownUserNoEnumeration(t *testing.T) {
	// 未知用户名场景下不把用户名写进审计，避免用户名枚举泄露。
	e := buildAuthAudit("auth.login.failed", "failed", nil, "10.0.0.2", "attempts=1")
	if e.ActorID != "" || e.ActorName != "" || e.TargetID != "" || e.TargetName != "" {
		t.Fatalf("unknown-user audit must not carry identity, got %+v", e)
	}
	if e.Outcome != "failed" {
		t.Fatalf("outcome wrong: %+v", e)
	}
}

func TestBuildAuthAudit_DetailPassthrough(t *testing.T) {
	u := &model.User{ID: uuid.New(), Username: "bob"}
	e := buildAuthAudit("auth.account.locked", "denied", u, "10.0.0.3", "threshold=5")
	if e.Detail != "threshold=5" {
		t.Fatalf("Detail = %q, want threshold=5", e.Detail)
	}
}
