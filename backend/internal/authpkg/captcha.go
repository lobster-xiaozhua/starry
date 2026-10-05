package authpkg

import (
	"context"
	"strings"
	"time"

	"github.com/mojocn/base64Captcha"
)

const captchaTTL = 5 * time.Minute

type RedisCaptchaStore struct {
	setFn   func(ctx context.Context, key string, value string, ttl time.Duration) error
	getFn   func(ctx context.Context, key string) (string, error)
	clearFn func(ctx context.Context, key string) error
}

func NewRedisCaptchaStore(
	set func(ctx context.Context, key string, value string, ttl time.Duration) error,
	get func(ctx context.Context, key string) (string, error),
	clear func(ctx context.Context, key string) error,
) base64Captcha.Store {
	return &RedisCaptchaStore{setFn: set, getFn: get, clearFn: clear}
}

func (s *RedisCaptchaStore) Set(id string, value string) error {
	return s.setFn(context.Background(), "captcha:"+id, value, captchaTTL)
}

func (s *RedisCaptchaStore) Get(id string, clear bool) string {
	ctx := context.Background()
	v, err := s.getFn(ctx, "captcha:"+id)
	if err != nil || v == "" {
		return ""
	}
	if clear {
		_ = s.clearFn(ctx, "captcha:"+id)
	}
	return v
}

func (s *RedisCaptchaStore) Verify(id, answer string, clear bool) bool {
	v := s.Get(id, clear)
	return strings.EqualFold(v, strings.TrimSpace(answer))
}

var _ base64Captcha.Store = (*RedisCaptchaStore)(nil)
