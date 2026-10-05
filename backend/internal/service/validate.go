package service

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode"

	"login-system/backend/internal/model"
)

type ValidationIssues map[string]string

var (
	ErrLocked          = errors.New("account locked")
	ErrBadCredentials  = errors.New("invalid credentials")
	ErrCaptchaRequired = errors.New("captcha required")
	ErrCaptchaInvalid  = errors.New("captcha invalid")
	ErrUserExists      = errors.New("user exists")
	ErrWeakPassword    = errors.New("weak password")
	ErrNotFound        = errors.New("not found")
)

func ValidatePassword(password string, s model.AuthSettings) ValidationIssues {
	issues := ValidationIssues{}
	if len(password) < s.PasswordMinLength {
		issues["password"] = fmt.Sprintf("密码长度至少 %d 位", s.PasswordMinLength)
		return issues
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}
	var categories int
	var missing []string
	if hasUpper {
		categories++
	} else if s.PasswordRequireUpper {
		missing = append(missing, "大写字母")
	}
	if hasLower {
		categories++
	} else if s.PasswordRequireLower {
		missing = append(missing, "小写字母")
	}
	if hasDigit {
		categories++
	} else if s.PasswordRequireDigit {
		missing = append(missing, "数字")
	}
	if hasSpecial {
		categories++
	} else if s.PasswordRequireSpecial {
		missing = append(missing, "特殊字符")
	}
	if categories < s.PasswordMinCategories {
		msg := fmt.Sprintf("密码需包含至少 %d 类字符", s.PasswordMinCategories)
		if len(missing) > 0 {
			msg += "（缺少：" + strings.Join(missing, "、") + "）"
		}
		issues["password"] = msg
	}
	return issues
}

func ValidateSettingsUpdate(in model.AuthSettings) ValidationIssues {
	issues := ValidationIssues{}
	if in.PasswordMinLength < 8 || in.PasswordMinLength > 128 {
		issues["passwordMinLength"] = "最小长度允许范围 8-128"
	}
	if in.PasswordMinCategories < 1 || in.PasswordMinCategories > 4 {
		issues["passwordMinCategories"] = "字符类别数允许范围 1-4"
	}
	if in.LockoutThreshold < 1 || in.LockoutThreshold > 20 {
		issues["lockoutThreshold"] = "锁定阈值允许范围 1-20"
	}
	if in.LockoutDurationMinutes < 1 || in.LockoutDurationMinutes > 1440 {
		issues["lockoutDurationMinutes"] = "锁定时长允许范围 1-1440 分钟"
	}
	if in.AccessTokenMinutes < 5 || in.AccessTokenMinutes > 120 {
		issues["accessTokenMinutes"] = "Access Token 有效期允许范围 5-120 分钟"
	}
	if in.RefreshTokenDays < 1 || in.RefreshTokenDays > 30 {
		issues["refreshTokenDays"] = "Refresh Token 有效期允许范围 1-30 天"
	}
	return issues
}

func LogResetToken(email, token string) {
	log.Printf("[DEV-MAIL] password reset token for %s: %s", email, token)
}
