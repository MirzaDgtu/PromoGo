package config

import (
	"net/url"
	"strings"
	"testing"
)

func validConfig() *Config {
	return &Config{
		App:      AppConfig{Env: "production"},
		Postgres: PostgresConfig{Host: "db", Port: 5432, User: "app", Password: "pw", DBName: "promogo", SSLMode: "require"},
		Redis:    RedisConfig{Addr: "redis:6379", TLSEnabled: true},
		Auth:     AuthConfig{AccessTokenSecret: strings.Repeat("s", 32)},
		SMS:      SMSConfig{Provider: "http", Endpoint: "https://sms.example.com/send", Token: "tok"},
		FCM:      FCMConfig{CredentialsJSON: "{}"},
		AntiFraud: AntiFraudConfig{
			DailyRedeemPointsLimit: 1000,
		},
	}
}

// TestPostgresConfig_DSN_EscapesSpecialCharacters is the regression test
// for building the DSN via fmt.Sprintf interpolation: a password containing
// URL-meaningful characters (@, :, /, #, %, ?) could be misparsed as part
// of the host/path/query, or smuggle extra connection parameters into the
// DSN. DSN must percent-encode credentials via net/url instead, and the
// result must round-trip through url.Parse back to the original values.
func TestPostgresConfig_DSN_EscapesSpecialCharacters(t *testing.T) {
	cases := []string{
		`p@ss:word/with#special?chars%20here`,
		`simple`,
		`p"'\<>&=+ pw`,
	}
	for _, password := range cases {
		cfg := PostgresConfig{Host: "db.internal", Port: 5432, User: "app_user", Password: password, DBName: "promogo", SSLMode: "require"}
		dsn := cfg.DSN()

		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("DSN() = %q produced an unparseable URL: %v", dsn, err)
		}
		if u.Scheme != "postgres" {
			t.Errorf("scheme = %q, want postgres", u.Scheme)
		}
		if u.Hostname() != "db.internal" {
			t.Errorf("host = %q, want db.internal (password characters leaked out of userinfo)", u.Hostname())
		}
		if u.Path != "/promogo" {
			t.Errorf("path = %q, want /promogo", u.Path)
		}
		gotPassword, _ := u.User.Password()
		if gotPassword != password {
			t.Errorf("round-tripped password = %q, want %q", gotPassword, password)
		}
		if got := u.Query().Get("sslmode"); got != "require" {
			t.Errorf("sslmode = %q, want require", got)
		}
	}
}

func TestValidateSMS_UnknownProviderAlwaysRejected(t *testing.T) {
	cfg := validConfig()
	cfg.App.Env = "development" // even in development — a strict enum, not a production-only check
	cfg.SMS.Provider = "sms-ru" // not a wired provider, however plausible-looking
	if err := cfg.validateSMS(); err == nil {
		t.Fatal("validateSMS() with an unrecognized provider error = nil, want an error even in development")
	}
}

func TestValidateSMS_LogRejectedOutsideDevelopment(t *testing.T) {
	cfg := validConfig()
	cfg.SMS.Provider = "log"
	if err := cfg.validateSMS(); err == nil {
		t.Fatal("validateSMS() with provider=log outside development error = nil, want an error")
	}
}

func TestValidateSMS_LogAllowedInDevelopment(t *testing.T) {
	cfg := validConfig()
	cfg.App.Env = "development"
	cfg.SMS.Provider = "log"
	cfg.SMS.Endpoint = ""
	cfg.SMS.Token = ""
	if err := cfg.validateSMS(); err != nil {
		t.Errorf("validateSMS() with provider=log in development error = %v, want nil", err)
	}
}

func TestValidateSMS_HTTPRequiresHTTPSEndpoint(t *testing.T) {
	cfg := validConfig()
	cfg.SMS.Endpoint = "http://sms.example.com/send"
	if err := cfg.validateSMS(); err == nil {
		t.Fatal("validateSMS() with a plain-HTTP endpoint outside development error = nil, want an error")
	}
}

func TestValidateSMS_HTTPMissingCredentialsRejected(t *testing.T) {
	cfg := validConfig()
	cfg.SMS.Token = ""
	if err := cfg.validateSMS(); err == nil {
		t.Fatal("validateSMS() with provider=http and no token error = nil, want an error")
	}
}

func TestValidatePostgres_DisableRejectedOutsideDevelopment(t *testing.T) {
	cfg := validConfig()
	cfg.Postgres.SSLMode = "disable"
	if err := cfg.validatePostgres(); err == nil {
		t.Fatal("validatePostgres() with sslmode=disable outside development error = nil, want an error")
	}
}

func TestValidatePostgres_PreferRejectedOutsideDevelopment(t *testing.T) {
	cfg := validConfig()
	cfg.Postgres.SSLMode = "prefer" // silently falls back to plaintext — not an actual TLS guarantee
	if err := cfg.validatePostgres(); err == nil {
		t.Fatal("validatePostgres() with sslmode=prefer outside development error = nil, want an error")
	}
}

func TestValidatePostgres_RequireAllowedOutsideDevelopment(t *testing.T) {
	cfg := validConfig()
	cfg.Postgres.SSLMode = "require"
	if err := cfg.validatePostgres(); err != nil {
		t.Errorf("validatePostgres() with sslmode=require error = %v, want nil", err)
	}
}

func TestValidatePostgres_DisableAllowedInDevelopment(t *testing.T) {
	cfg := validConfig()
	cfg.App.Env = "development"
	cfg.Postgres.SSLMode = "disable"
	if err := cfg.validatePostgres(); err != nil {
		t.Errorf("validatePostgres() with sslmode=disable in development error = %v, want nil", err)
	}
}

func TestValidateRedis_TLSRequiredOutsideDevelopment(t *testing.T) {
	cfg := validConfig()
	cfg.Redis.TLSEnabled = false
	if err := cfg.validateRedis(); err == nil {
		t.Fatal("validateRedis() with tls_enabled=false outside development error = nil, want an error")
	}
}

func TestValidateRedis_TLSNotRequiredInDevelopment(t *testing.T) {
	cfg := validConfig()
	cfg.App.Env = "development"
	cfg.Redis.TLSEnabled = false
	if err := cfg.validateRedis(); err != nil {
		t.Errorf("validateRedis() with tls_enabled=false in development error = %v, want nil", err)
	}
}
