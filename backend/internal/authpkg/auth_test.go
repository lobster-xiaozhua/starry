package authpkg

import (
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if !CheckPassword(hash, "s3cret-pass") {
		t.Fatal("正确密码应校验通过")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("错误密码应校验失败")
	}
}

// 访问令牌应可在同一密钥下正确解析，且密钥不符时被拒绝。
func TestAccessTokenRoundTrip(t *testing.T) {
	secret := "test-secret"
	access, _, err := IssueAccessToken(secret, "user-1", "alice", "admin", time.Minute)
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	claims, err := ParseAccessToken(secret, access)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if claims.UserID != "user-1" || claims.Role != "admin" {
		t.Fatalf("claims 不符: %+v", claims)
	}
	if _, err := ParseAccessToken("other-secret", access); err == nil {
		t.Fatal("错误密钥应被拒绝")
	}
}
