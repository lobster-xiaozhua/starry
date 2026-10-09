-- Initial schema for users and auth_settings tables.
-- Extracted from GORM AutoMigrate to enable versioned migrations.
-- Compatible with the schema produced by model.User and model.AuthSettings.

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    username VARCHAR(64) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(100) NOT NULL,
    role VARCHAR(16) NOT NULL DEFAULT 'user',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email);

CREATE TABLE IF NOT EXISTS auth_settings (
    id BIGINT PRIMARY KEY,
    password_min_length INTEGER NOT NULL,
    password_require_upper BOOLEAN NOT NULL,
    password_require_lower BOOLEAN NOT NULL,
    password_require_digit BOOLEAN NOT NULL,
    password_require_special BOOLEAN NOT NULL,
    password_min_categories INTEGER NOT NULL,
    lockout_threshold INTEGER NOT NULL,
    lockout_duration_minutes INTEGER NOT NULL,
    access_token_minutes INTEGER NOT NULL,
    refresh_token_days INTEGER NOT NULL,
    captcha_enabled BOOLEAN NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
