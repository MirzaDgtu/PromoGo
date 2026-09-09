package config

import (
	"bufio"
	"fmt"
	"net/url"
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
	App       AppConfig       `mapstructure:"app"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	Postgres  PostgresConfig  `mapstructure:"postgres"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Logger    LoggerConfig    `mapstructure:"logger"`
	FCM       FCMConfig       `mapstructure:"fcm"`
	SMS       SMSConfig       `mapstructure:"sms"`
	Auth      AuthConfig      `mapstructure:"auth"`
	OIDC      OIDCConfig      `mapstructure:"oidc"`
	RateLimit RateLimitConfig `mapstructure:"ratelimit"`
	AntiFraud AntiFraudConfig `mapstructure:"antifraud"`
}

// AppConfig holds general application metadata.
type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`

	// SkipStartupMigrations, when true, makes app.New verify the schema has
	// no pending migrations (internal/migrate.Verify) instead of applying
	// them (internal/migrate.Run). Required for a multi-replica production
	// deployment: migrations become a separate step (`promogo migrate`, run
	// once before any replica of the new version starts — see
	// docs/deployment.md) instead of a race every replica independently
	// enters at startup. Defaults to false so a single-instance/dev
	// deployment (docker-compose) keeps auto-migrating, matching this
	// repo's existing behavior.
	SkipStartupMigrations bool `mapstructure:"skip_startup_migrations"`
}

// HTTPConfig configures the application's HTTP server.
type HTTPConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`

	// TrustedProxies lists CIDR ranges (e.g. "10.0.0.0/8") of reverse
	// proxies/load balancers allowed to set X-Forwarded-For. Empty (the
	// default) means no proxy is trusted: rate limiting's IP dimension uses
	// the TCP connection's RemoteAddr only, matching clientIP's existing
	// behavior for audit logging and OTP rate limiting — an arbitrary
	// client must never be able to pick its own rate-limit bucket by
	// spoofing this header.
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

// RateLimitConfig bounds the distributed (Redis-backed) rate limiter's
// profiles — see internal/ratelimit and httpserver's routes.go
// RateLimitProfile. All limits are requests allowed per Window; defaults
// are deliberately generous (especially for the 1C-facing accrual/lookup
// profiles) so a legitimate integration is never throttled under normal
// operation — they exist to bound abuse, not to shape steady-state traffic.
type RateLimitConfig struct {
	StaffLoginIPLimit  int           `mapstructure:"staff_login_ip_limit"`
	StaffLoginIPWindow time.Duration `mapstructure:"staff_login_ip_window"`

	AdminIPLimit     int           `mapstructure:"admin_ip_limit"`
	AdminIPWindow    time.Duration `mapstructure:"admin_ip_window"`
	AdminStaffLimit  int           `mapstructure:"admin_staff_limit"`
	AdminStaffWindow time.Duration `mapstructure:"admin_staff_window"`

	ClientLookupIPLimit         int           `mapstructure:"client_lookup_ip_limit"`
	ClientLookupIPWindow        time.Duration `mapstructure:"client_lookup_ip_window"`
	ClientLookupPrincipalLimit  int           `mapstructure:"client_lookup_principal_limit"`
	ClientLookupPrincipalWindow time.Duration `mapstructure:"client_lookup_principal_window"`
	ClientLookupPhoneLimit      int           `mapstructure:"client_lookup_phone_limit"`
	ClientLookupPhoneWindow     time.Duration `mapstructure:"client_lookup_phone_window"`

	AccrualIPLimit         int           `mapstructure:"accrual_ip_limit"`
	AccrualIPWindow        time.Duration `mapstructure:"accrual_ip_window"`
	AccrualPrincipalLimit  int           `mapstructure:"accrual_principal_limit"`
	AccrualPrincipalWindow time.Duration `mapstructure:"accrual_principal_window"`
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

// DSN returns a connection string suitable for pgxpool. Built via net/url
// rather than fmt.Sprintf interpolation: User/Password are percent-encoded
// through url.UserPassword, so a credential containing a URL-meaningful
// character (@, :, /, ?, #, %) can never be misparsed as part of the host,
// path, or query string — or, worse, used to smuggle extra connection
// parameters into the DSN.
func (c PostgresConfig) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   "/" + c.DBName,
	}
	q := url.Values{}
	q.Set("sslmode", c.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

// RedisConfig configures the connection to Redis. TLSEnabled is required
// outside development (see Load's validateRedis) — Redis holds OTP
// challenges, rate-limit counters, and QR one-time tokens, all sensitive
// enough that the connection to it shouldn't be plaintext on an untrusted
// network.
type RedisConfig struct {
	Addr       string `mapstructure:"addr"`
	Password   string `mapstructure:"password"`
	DB         int    `mapstructure:"db"`
	TLSEnabled bool   `mapstructure:"tls_enabled"`
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

// SMSConfig configures OTP/notification SMS delivery. No specific vendor is
// hardcoded (none has been chosen for the project yet — see
// .claude/skills/add-notification-channel/SKILL.md): Provider "log" uses
// internal/notification/logsms (development/test only); Provider "http"
// posts to Endpoint via internal/notification/httpsms, a generic HTTPS
// gateway contract. Token is set via PROMOGO_SMS_TOKEN, never committed to
// configs/config.yaml. See Load's validateSMS for the production fail-fast
// rule.
type SMSConfig struct {
	Provider string        `mapstructure:"provider"`
	Endpoint string        `mapstructure:"endpoint"`
	Token    string        `mapstructure:"token"`
	Sender   string        `mapstructure:"sender"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// AntiFraudConfig bounds the redemption daily limit and QR TTL/cooldowns
// (see DEC-011, DEC-013). DailyRedeemPointsLimit has no default outside
// development — see Load's validateAntiFraud.
type AntiFraudConfig struct {
	DailyRedeemPointsLimit int64         `mapstructure:"daily_redeem_points_limit"`
	DailyRedeemWindow      time.Duration `mapstructure:"daily_redeem_window"`

	QRTTL             time.Duration `mapstructure:"qr_ttl"`
	QRIssueCooldown   time.Duration `mapstructure:"qr_issue_cooldown"`
	QRConsumeCooldown time.Duration `mapstructure:"qr_consume_cooldown"`
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
	if err := cfg.validateAntiFraud(); err != nil {
		return nil, err
	}
	if err := cfg.validateSMS(); err != nil {
		return nil, err
	}
	if err := cfg.validateFCM(); err != nil {
		return nil, err
	}
	if err := cfg.validatePostgres(); err != nil {
		return nil, err
	}
	if err := cfg.validateRedis(); err != nil {
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

// validateAntiFraud fails fast if the daily redemption limit is unset
// outside development — an anti-fraud limit that silently defaults to "no
// limit" in production would defeat DEC-013 without any visible error.
func (c *Config) validateAntiFraud() error {
	if c.App.Env == "development" {
		return nil
	}
	if c.AntiFraud.DailyRedeemPointsLimit <= 0 {
		return fmt.Errorf("antifraud.daily_redeem_points_limit must be set (> 0) via PROMOGO_ANTIFRAUD_DAILY_REDEEM_POINTS_LIMIT outside development")
	}
	return nil
}

// validSMSProviders is the strict enum for sms.provider — any other value
// fails Load outright (see validateSMS) rather than reaching internal/app,
// where an unrecognized value used to silently construct the dev-only log
// sender in every environment, production included.
var validSMSProviders = map[string]bool{"log": true, "http": true}

// validateSMS enforces sms.provider as a strict enum in every environment
// (an unrecognized value is always a startup error, never a silent
// fallback — see DEC-014), and outside development additionally fails fast
// if the provider is still the dev-only "log" stub, or if provider "http"
// is missing the endpoint/token it needs or the endpoint isn't HTTPS — an
// OTP code sent over plain HTTP would be interceptable in transit.
func (c *Config) validateSMS() error {
	if !validSMSProviders[c.SMS.Provider] {
		return fmt.Errorf("sms.provider %q is not a recognized value (must be \"log\" or \"http\")", c.SMS.Provider)
	}
	if c.App.Env == "development" {
		return nil
	}
	if c.SMS.Provider == "log" {
		return fmt.Errorf("sms.provider must not be \"log\" outside development (set PROMOGO_SMS_PROVIDER=http and configure sms.endpoint/sms.token)")
	}
	if c.SMS.Endpoint == "" || c.SMS.Token == "" {
		return fmt.Errorf("sms.endpoint and sms.token must be set via PROMOGO_SMS_ENDPOINT/PROMOGO_SMS_TOKEN when sms.provider=http")
	}
	if !strings.HasPrefix(c.SMS.Endpoint, "https://") {
		return fmt.Errorf("sms.endpoint must use https:// outside development, got %q", c.SMS.Endpoint)
	}
	return nil
}

// validateFCM fails fast outside development if FCM credentials aren't
// configured — production must not silently fall back to the log-based
// notification channel (see DEC-014).
func (c *Config) validateFCM() error {
	if c.App.Env == "development" {
		return nil
	}
	if c.FCM.CredentialsJSON == "" {
		return fmt.Errorf("fcm.credentials_json must be set via PROMOGO_FCM_CREDENTIALS_JSON outside development")
	}
	return nil
}

// pgSSLModesRequiringTLS are the postgres.sslmode values that actually
// enforce an encrypted connection. "prefer"/"allow" silently fall back to
// plaintext if the server doesn't offer TLS, which defeats the point of a
// production requirement, so they don't count here.
var pgSSLModesRequiringTLS = map[string]bool{"require": true, "verify-ca": true, "verify-full": true}

// validatePostgres fails fast outside development unless postgres.sslmode
// is set to a mode that actually enforces TLS — "disable" (the default,
// for local dev) and the fallback-permitting "allow"/"prefer" modes would
// otherwise let a production deployment silently run an unencrypted
// connection to its primary datastore.
func (c *Config) validatePostgres() error {
	if c.App.Env == "development" {
		return nil
	}
	if !pgSSLModesRequiringTLS[c.Postgres.SSLMode] {
		return fmt.Errorf("postgres.sslmode must be \"require\", \"verify-ca\", or \"verify-full\" outside development, got %q", c.Postgres.SSLMode)
	}
	return nil
}

// validateRedis fails fast outside development unless redis.tls_enabled is
// set — see RedisConfig's doc comment.
func (c *Config) validateRedis() error {
	if c.App.Env == "development" {
		return nil
	}
	if !c.Redis.TLSEnabled {
		return fmt.Errorf("redis.tls_enabled must be true outside development (set PROMOGO_REDIS_TLS_ENABLED=true)")
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
	v.SetDefault("app.skip_startup_migrations", false)

	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)

	v.SetDefault("postgres.host", "localhost")
	v.SetDefault("postgres.port", 5432)
	v.SetDefault("postgres.sslmode", "disable")
	v.SetDefault("postgres.max_conns", 10)

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.tls_enabled", false)

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

	// Rate limiting: generous, especially on the 1C-facing accrual/lookup
	// profiles — these bound abuse, they must not throttle normal
	// integration traffic. See RateLimitConfig's doc comment.
	v.SetDefault("ratelimit.staff_login_ip_limit", 20)
	v.SetDefault("ratelimit.staff_login_ip_window", 5*time.Minute)

	v.SetDefault("ratelimit.admin_ip_limit", 120)
	v.SetDefault("ratelimit.admin_ip_window", time.Minute)
	v.SetDefault("ratelimit.admin_staff_limit", 300)
	v.SetDefault("ratelimit.admin_staff_window", time.Minute)

	v.SetDefault("ratelimit.client_lookup_ip_limit", 120)
	v.SetDefault("ratelimit.client_lookup_ip_window", time.Minute)
	v.SetDefault("ratelimit.client_lookup_principal_limit", 300)
	v.SetDefault("ratelimit.client_lookup_principal_window", time.Minute)
	v.SetDefault("ratelimit.client_lookup_phone_limit", 30)
	v.SetDefault("ratelimit.client_lookup_phone_window", time.Minute)

	v.SetDefault("ratelimit.accrual_ip_limit", 300)
	v.SetDefault("ratelimit.accrual_ip_window", time.Minute)
	v.SetDefault("ratelimit.accrual_principal_limit", 600)
	v.SetDefault("ratelimit.accrual_principal_window", time.Minute)

	v.SetDefault("sms.provider", "log")
	v.SetDefault("sms.timeout", 5*time.Second)

	// Anti-fraud: daily_redeem_points_limit has a documented development-only
	// default (1000) — validateAntiFraud requires it to be explicitly set
	// outside development. QR TTL/cooldowns default in every environment
	// (see DEC-011).
	v.SetDefault("antifraud.daily_redeem_points_limit", 1000)
	v.SetDefault("antifraud.daily_redeem_window", 24*time.Hour)
	v.SetDefault("antifraud.qr_ttl", 2*time.Minute)
	v.SetDefault("antifraud.qr_issue_cooldown", 30*time.Second)
	v.SetDefault("antifraud.qr_consume_cooldown", time.Second)
}
