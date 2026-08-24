package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	App    AppConfig
	DB     DBConfig
	JWT    JWTConfig
	OTP    OTPConfig
	SMTP   SMTPConfig
	MinIO  MinIOConfig
	Seeder SeederConfig
	Log    LogConfig
	Valkey ValkeyConfig
}

type AppConfig struct {
	Env  string
	Port string
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	Path     string
}

type JWTConfig struct {
	Secret         string
	ExpiryMinutes  int
}

type OTPConfig struct {
	TTLMinutes int
}

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Bucket    string
	PublicURL string
}

type SeederConfig struct {
	Email    string
	Password string
}

type LogConfig struct {
	Level    string
	Format   string
	FilePath string
}

type ValkeyConfig struct {
	Addr     string
	Password string
	DB       int
}

// Load reads configuration from environment variables and .env file.
// Environment variables take precedence over .env file values.
func Load() Config {
	godotenv.Load()

	return Config{
		App: AppConfig{
			Env:  getEnv("APP_ENV", "development"),
			Port: getEnv("APP_PORT", "8080"),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "backend"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
			Path:     getEnv("DB_PATH", "public"),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", ""),
			ExpiryMinutes: getEnvInt("JWT_EXPIRY_MINUTES", 60),
		},
		OTP: OTPConfig{
			TTLMinutes: getEnvInt("OTP_TTL_MINUTES", 5),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", "smtp.gmail.com"),
			Port:     getEnv("SMTP_PORT", "587"),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", ""),
		},
		MinIO: MinIOConfig{
			Endpoint:  getEnv("MINIO_ENDPOINT", "minio:9000"),
			AccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin123"),
			UseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
			Bucket:    getEnv("MINIO_BUCKET", "backend"),
			PublicURL: getEnv("MINIO_PUBLIC_URL", ""),
		},
		Seeder: SeederConfig{
			Email:    getEnv("SEEDER_EMAIL", ""),
			Password: getEnv("SEEDER_PASSWORD", ""),
		},
		Log: LogConfig{
			Level:    getEnv("LOG_LEVEL", "info"),
			Format:   getEnv("LOG_FORMAT", "json"),
			FilePath: getEnv("LOG_FILE_PATH", "/app/logs/app.log"),
		},
		Valkey: ValkeyConfig{
			Addr:     getEnv("VALKEY_ADDR", "valkey:6379"),
			Password: getEnv("VALKEY_PASSWORD", ""),
			DB:       getEnvInt("VALKEY_DB", 0),
		},
	}
}

func getEnv(key, fallback string) string {
	if val := getenv(key); val != "" {
		return val
	}
	return fallback
}

func getenv(key string) string {
	return os.Getenv(key)
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}
