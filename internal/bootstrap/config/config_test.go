package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	env := map[string]string{
		"APP_ENV":            "development",
		"APP_PORT":           "3000",
		"DB_HOST":            "localhost",
		"DB_PORT":            "5432",
		"DB_USER":            "testuser",
		"DB_PASSWORD":        "testpass",
		"DB_NAME":            "testdb",
		"DB_SSL_MODE":        "require",
		"DB_PATH":            "public",
		"JWT_SECRET":         "supersecret",
		"JWT_EXPIRY_MINUTES": "30",
		"OTP_TTL_MINUTES":    "10",
		"SMTP_HOST":          "smtp.gmail.com",
		"SMTP_PORT":          "587",
		"SMTP_USERNAME":      "user@gmail.com",
		"SMTP_PASSWORD":      "apppassword",
		"SMTP_FROM":          "Invise <user@gmail.com>",
		"MINIO_ENDPOINT":     "minio:9000",
		"MINIO_ACCESS_KEY":   "minioadmin",
		"MINIO_SECRET_KEY":   "minioadmin123",
		"MINIO_USE_SSL":      "true",
		"MINIO_BUCKET":       "mybucket",
		"MINIO_PUBLIC_URL":   "https://example.com/storage",
		"SEEDER_EMAIL":       "admin@example.com",
		"SEEDER_PASSWORD":    "seedpass",
		"LOG_LEVEL":          "debug",
		"LOG_FORMAT":         "text",
		"LOG_FILE_PATH":      "/tmp/test.log",
	}

	for k, v := range env {
		t.Setenv(k, v)
	}

	cfg := Load()

	if cfg.App.Env != "development" {
		t.Errorf("App.Env = %q, want %q", cfg.App.Env, "development")
	}
	if cfg.App.Port != "3000" {
		t.Errorf("App.Port = %q, want %q", cfg.App.Port, "3000")
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("DB.Host = %q, want %q", cfg.DB.Host, "localhost")
	}
	if cfg.DB.Port != "5432" {
		t.Errorf("DB.Port = %q, want %q", cfg.DB.Port, "5432")
	}
	if cfg.DB.User != "testuser" {
		t.Errorf("DB.User = %q, want %q", cfg.DB.User, "testuser")
	}
	if cfg.DB.Password != "testpass" {
		t.Errorf("DB.Password = %q, want %q", cfg.DB.Password, "testpass")
	}
	if cfg.DB.Name != "testdb" {
		t.Errorf("DB.Name = %q, want %q", cfg.DB.Name, "testdb")
	}
	if cfg.DB.SSLMode != "require" {
		t.Errorf("DB.SSLMode = %q, want %q", cfg.DB.SSLMode, "require")
	}
	if cfg.DB.Path != "public" {
		t.Errorf("DB.Path = %q, want %q", cfg.DB.Path, "public")
	}
	if cfg.JWT.Secret != "supersecret" {
		t.Errorf("JWT.Secret = %q, want %q", cfg.JWT.Secret, "supersecret")
	}
	if cfg.JWT.ExpiryMinutes != 30 {
		t.Errorf("JWT.ExpiryMinutes = %d, want %d", cfg.JWT.ExpiryMinutes, 30)
	}
	if cfg.OTP.TTLMinutes != 10 {
		t.Errorf("OTP.TTLMinutes = %d, want %d", cfg.OTP.TTLMinutes, 10)
	}
	if cfg.SMTP.Host != "smtp.gmail.com" {
		t.Errorf("SMTP.Host = %q, want %q", cfg.SMTP.Host, "smtp.gmail.com")
	}
	if cfg.SMTP.Port != "587" {
		t.Errorf("SMTP.Port = %q, want %q", cfg.SMTP.Port, "587")
	}
	if cfg.SMTP.Username != "user@gmail.com" {
		t.Errorf("SMTP.Username = %q, want %q", cfg.SMTP.Username, "user@gmail.com")
	}
	if cfg.SMTP.Password != "apppassword" {
		t.Errorf("SMTP.Password = %q, want %q", cfg.SMTP.Password, "apppassword")
	}
	if cfg.SMTP.From != "Invise <user@gmail.com>" {
		t.Errorf("SMTP.From = %q, want %q", cfg.SMTP.From, "Invise <user@gmail.com>")
	}
	if cfg.MinIO.Endpoint != "minio:9000" {
		t.Errorf("MinIO.Endpoint = %q, want %q", cfg.MinIO.Endpoint, "minio:9000")
	}
	if cfg.MinIO.AccessKey != "minioadmin" {
		t.Errorf("MinIO.AccessKey = %q, want %q", cfg.MinIO.AccessKey, "minioadmin")
	}
	if cfg.MinIO.SecretKey != "minioadmin123" {
		t.Errorf("MinIO.SecretKey = %q, want %q", cfg.MinIO.SecretKey, "minioadmin123")
	}
	if !cfg.MinIO.UseSSL {
		t.Error("MinIO.UseSSL = false, want true")
	}
	if cfg.MinIO.Bucket != "mybucket" {
		t.Errorf("MinIO.Bucket = %q, want %q", cfg.MinIO.Bucket, "mybucket")
	}
	if cfg.MinIO.PublicURL != "https://example.com/storage" {
		t.Errorf("MinIO.PublicURL = %q, want %q", cfg.MinIO.PublicURL, "https://example.com/storage")
	}
	if cfg.Seeder.Email != "admin@example.com" {
		t.Errorf("Seeder.Email = %q, want %q", cfg.Seeder.Email, "admin@example.com")
	}
	if cfg.Seeder.Password != "seedpass" {
		t.Errorf("Seeder.Password = %q, want %q", cfg.Seeder.Password, "seedpass")
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
	if cfg.Log.Format != "text" {
		t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, "text")
	}
	if cfg.Log.FilePath != "/tmp/test.log" {
		t.Errorf("Log.FilePath = %q, want %q", cfg.Log.FilePath, "/tmp/test.log")
	}
	if cfg.Valkey.Host != "valkey" {
		t.Errorf("Valkey.Host = %q, want %q", cfg.Valkey.Host, "valkey")
	}
	if cfg.Valkey.Port != "6379" {
		t.Errorf("Valkey.Port = %q, want %q", cfg.Valkey.Port, "6379")
	}
}

func TestLoadDefaults(t *testing.T) {
	// Only set required fields
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_PASSWORD", "p")
	t.Setenv("DB_NAME", "d")
	t.Setenv("JWT_SECRET", "s")
	t.Setenv("SEEDER_EMAIL", "e")
	t.Setenv("SEEDER_PASSWORD", "p")

	cfg := Load()

	if cfg.JWT.ExpiryMinutes != 60 {
		t.Errorf("JWT.ExpiryMinutes = %d, want default 60", cfg.JWT.ExpiryMinutes)
	}
	if cfg.OTP.TTLMinutes != 5 {
		t.Errorf("OTP.TTLMinutes = %d, want default 5", cfg.OTP.TTLMinutes)
	}
}

func TestLoadFromEnvFile(t *testing.T) {
	dir := t.TempDir()
	envFile := dir + "/.env"
	content := "APP_ENV=staging\nAPP_PORT=9090\nJWT_SECRET=filesecret\nDB_HOST=localhost\nDB_PORT=5432\nDB_USER=u\nDB_PASSWORD=p\nDB_NAME=d\nSEEDER_EMAIL=e\nSEEDER_PASSWORD=p\n"
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp dir so godotenv finds the .env file
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	// Clear env vars so .env file values are used
	os.Unsetenv("APP_ENV")
	os.Unsetenv("APP_PORT")
	os.Unsetenv("JWT_SECRET")

	cfg := Load()

	if cfg.App.Env != "staging" {
		t.Errorf("App.Env = %q, want %q (from .env file)", cfg.App.Env, "staging")
	}
	if cfg.App.Port != "9090" {
		t.Errorf("App.Port = %q, want %q (from .env file)", cfg.App.Port, "9090")
	}
	if cfg.JWT.Secret != "filesecret" {
		t.Errorf("JWT.Secret = %q, want %q (from .env file)", cfg.JWT.Secret, "filesecret")
	}
}
