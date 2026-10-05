package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
)

type Config struct {
	Port          string
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	JWTSecret     string
	AdminUsername string
	AdminEmail    string
	UploadDir     string
}

func Load() *Config {
	return &Config{
		Port:          envOr("PORT", "8080"),
		DBHost:        envOr("DB_HOST", "127.0.0.1"),
		DBPort:        envOr("DB_PORT", "5432"),
		DBUser:        envOr("DB_USER", "login_user"),
		DBPassword:    envOr("DB_PASSWORD", "login_pass_2026"),
		DBName:        envOr("DB_NAME", "login_system"),
		RedisAddr:     envOr("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       0,
		JWTSecret:     envOr("JWT_SECRET", generateRandomSecret()),
		AdminUsername: envOr("ADMIN_USERNAME", "admin"),
		AdminEmail:    envOr("ADMIN_EMAIL", "admin@example.com"),
		UploadDir:     envOr("UPLOAD_DIR", "uploads"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func generateRandomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatal("failed to generate JWT secret: ", err)
	}
	return hex.EncodeToString(b)
}
