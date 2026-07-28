package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application, loaded from a YAML
// file and overridable via PROMOGO_-prefixed environment variables
// (e.g. PROMOGO_POSTGRES_PASSWORD overrides postgres.password).
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	HTTP     HTTPConfig     `mapstructure:"http"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Logger   LoggerConfig   `mapstructure:"logger"`
	FCM      FCMConfig      `mapstructure:"fcm"`
	Auth     AuthConfig     `mapstructure:"auth"`
	OIDC     OIDCConfig     `mapstructure:"oidc"`
}

// AppConfig holds general application metadata.
type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

// HTTPConfig configures the application's HTTP server.
type HTTPConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// Addr returns the host:port address the HTTP server should bind to.
func (c HTTPConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// PostgresConfig configures the connection to PostgreSQL.
type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	MaxConns int32  `mapstructure:"max_conns"`
}

// DSN returns a connection string suitable for pgxpool.
func (c PostgresConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode)
}

// RedisConfig configures the connection to Redis.
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// LoggerConfig configures the application's structured logger.
type LoggerConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// FCMConfig configures Firebase Cloud Messaging for mobile push
// notifications on point accrual. CredentialsJSON is the raw contents of a
// Firebase service-account key file. Set via PROMOGO_FCM_CREDENTIALS_JSON,
// never committed to configs/config.yaml. An empty CredentialsJSON falls
// back to the log-based notification channel (internal/notification/logchannel).
type FCMConfig struct {
	CredentialsJSON string `mapstructure:"credentials_json"`
}

// AuthConfig configures customer OTP/session auth and staff access-token
// issuance. AccessTokenSecret is the only field with no default — set via
// PROMOGO_AUTH_ACCESS_TOKEN_SECRET, never committed to configs/config.yaml
// (see Load's validation).
type AuthConfig struct {
	AccessTokenSecret string        `mapstructure:"access_token_secret"`
	AccessTokenTTL    time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL   time.Duration `mapstructure:"refresh_token_ttl"`

	OTPTTL                 time.Duration `mapstructure:"otp_ttl"`
	OTPResendCooldown      time.Duration `mapstructure:"otp_resend_cooldown"`
	OTPMaxAttempts         int           `mapstructure:"otp_max_attempts"`
	OTPRateLimitWindow     time.Duration `mapstructure:"otp_rate_limit_window"`
	OTPMaxRequestsPerPhone int           `mapstructure:"otp_max_requests_per_phone"`
	OTPMaxRequestsPerIP    int           `mapstructure:"otp_max_requests_per_ip"`
}

// OIDCConfig configures staff authentication against the retailer/
// platform's OIDC identity provider (see internal/auth/oidc.go). Any
// OIDC-compliant IdP works; there is no PromoGo-specific default. Unset
// (empty IssuerURL) disables staff OIDC login — /api/v1/staff/auth/oidc
// will fail JWKS verification until configured.
type OIDCConfig struct {
	IssuerURL    string        `mapstructure:"issuer_url"`
	Audience     string        `mapstructure:"audience"`
	JWKSURL      string        `mapstructure:"jwks_url"`
	JWKSCacheTTL time.Duration `mapstructure:"jwks_cache_ttl"`
}

// Load reads configuration from the YAML file at path, applying defaults
// and environment variable overrides.
func Load(path string) (*Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(path)

	v.SetEnvPrefix("promogo")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.validateAuth(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validateAuth fails fast if AccessTokenSecret is missing or weak outside
// local development — a service that silently ran with an empty/guessable
// JWT signing secret would be a critical, easy-to-miss vulnerability rather
// than a loud startup failure.
func (c *Config) validateAuth() error {
	if c.App.Env == "development" {
		return nil
	}
	if len(c.Auth.AccessTokenSecret) < 32 {
		return fmt.Errorf("auth.access_token_secret must be set (>= 32 bytes) via PROMOGO_AUTH_ACCESS_TOKEN_SECRET outside development")
	}
	return nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}

	return scanner.Err()
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "promogo")
	v.SetDefault("app.env", "development")

	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)

	v.SetDefault("postgres.host", "localhost")
	v.SetDefault("postgres.port", 5432)
	v.SetDefault("postgres.sslmode", "disable")
	v.SetDefault("postgres.max_conns", 10)

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)

	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.format", "json")

	v.SetDefault("auth.access_token_ttl", 15*time.Minute)
	v.SetDefault("auth.refresh_token_ttl", 30*24*time.Hour)
	v.SetDefault("auth.otp_ttl", 5*time.Minute)
	v.SetDefault("auth.otp_resend_cooldown", 60*time.Second)
	v.SetDefault("auth.otp_max_attempts", 5)
	v.SetDefault("auth.otp_rate_limit_window", time.Hour)
	v.SetDefault("auth.otp_max_requests_per_phone", 5)
	v.SetDefault("auth.otp_max_requests_per_ip", 20)

	v.SetDefault("oidc.jwks_cache_ttl", 10*time.Minute)
}
