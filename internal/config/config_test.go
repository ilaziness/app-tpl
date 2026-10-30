package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ilaziness/app-tpl/internal/constant"
)

// testdataDir 返回 testdata 目录路径
func testdataDir() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "testdata")
}

// mustLoad 从 testdata 加载配置，失败则 panic
func mustLoad(t *testing.T, filename string) *Config {
	t.Helper()
	t.Setenv(EnvVarAppEnv, "")
	cfg, err := LoadWithEnv(filepath.Join(testdataDir(), filename), "")
	if err != nil {
		t.Fatalf("failed to load config %q: %v", filename, err)
	}
	return cfg
}

// assertError 断言期望的错误是否匹配
func assertError(t *testing.T, err error, wantErr bool, errMsg string) {
	t.Helper()
	if wantErr {
		if err == nil {
			t.Error("expected error but got nil")
			return
		}
		if errMsg != "" && err.Error() != errMsg {
			t.Errorf("error message = %q, want %q", err.Error(), errMsg)
		}
	} else if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestConfigValidate 验证配置校验逻辑
func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		// 有效配置
		{"valid full config", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}}, false, ""},
		{"valid minimal config", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}}, false, ""},
		{"valid with empty log level", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Log: LogConfig{Level: ""}}, false, ""},
		{"valid log levels debug", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Log: LogConfig{Level: "debug"}}, false, ""},
		{"valid log levels DEBUG", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Log: LogConfig{Level: "DEBUG"}}, false, ""},
		{"valid log levels warn", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Log: LogConfig{Level: "warn"}}, false, ""},
		{"valid log levels error", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Log: LogConfig{Level: "error"}}, false, ""},
		{"valid sqlite3 without host", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: constant.DriverSQLite3, Host: "", Database: "test.db"}}, false, ""},
		{"redis disabled skip validation", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Redis: RedisConfig{Enabled: false}}, false, ""},
		{"valid redis enabled", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Redis: RedisConfig{Enabled: true, Host: "localhost"}}, false, ""},

		// 无效配置
		{"missing app name", Config{HTTP: HTTPConfig{Port: 8080}}, true, "app.name is required"},
		{"http disabled ignores port", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Enabled: false, Port: 0}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}}, false, ""},
		{"port zero", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Enabled: true, Port: 0}}, true, "http.port must be between 1 and 65535"},
		{"port too high", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Enabled: true, Port: 70000}}, true, "http.port must be between 1 and 65535"},
		{"invalid log level", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Log: LogConfig{Level: "invalid"}}, true, "log.level must be one of: debug, info, warn, error"},
		{"database missing driver", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "", Database: "test.db"}}, true, "database.driver is required"},
		{"database mysql no host", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: constant.DriverMySQL, Host: "", Database: "testdb"}}, true, "database.host is required"},
		{"database postgres no host", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: constant.DriverPostgreSQL, Host: "", Database: "testdb"}}, true, "database.host is required"},
		{"database no database name", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "mysql", Host: "localhost"}}, true, "database.database is required"},
		{"redis enabled no host", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Redis: RedisConfig{Enabled: true, Host: ""}}, true, "redis.host is required when redis is enabled"},
		{"valid tcp enabled", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, TCP: TCPConfig{Enabled: true, Port: 9000, Codec: "json", MaxConnections: 100}}, false, ""},
		{"tcp enabled invalid port", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, TCP: TCPConfig{Enabled: true, Port: 0}}, true, "tcp.port must be between 1 and 65535"},
		{"tcp enabled invalid codec", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, TCP: TCPConfig{Enabled: true, Port: 9000, Codec: "invalid"}}, true, "tcp.codec must be one of: json, binary"},
		{"tcp enabled invalid max connections", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, TCP: TCPConfig{Enabled: true, Port: 9000, Codec: "json", MaxConnections: 0}}, true, "tcp.max_connections must be positive"},
		{"valid udp enabled", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, UDP: UDPConfig{Enabled: true, Port: 9001, Codec: "json", WorkerPoolSize: 10}}, false, ""},
		{"udp enabled invalid port", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, UDP: UDPConfig{Enabled: true, Port: 0}}, true, "udp.port must be between 1 and 65535"},
		{"udp enabled invalid codec", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, UDP: UDPConfig{Enabled: true, Port: 9001, Codec: "invalid"}}, true, "udp.codec must be one of: json, binary"},
		{"udp enabled invalid worker pool size", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, UDP: UDPConfig{Enabled: true, Port: 9001, Codec: "json", WorkerPoolSize: 0}}, true, "udp.worker_pool_size must be positive"},

		// 枚举校验
		{"invalid app env", Config{App: AppConfig{Name: "app", Env: "staging"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}}, true, "app.env must be one of: dev, prod, test"},
		{"invalid log format", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Log: LogConfig{Format: "yaml"}}, true, "log.format must be one of: json, console"},
		{"invalid log output", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Log: LogConfig{Output: "stderr"}}, true, "log.output must be one of: stdout, file, both"},
		{"log output file missing filename", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}, Log: LogConfig{Output: "file"}}, true, "log.filename is required when log.output is file or both"},
		{"invalid database driver", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "oracle", Host: "localhost", Database: "test.db"}}, true, "database.driver must be one of: sqlite, sqlite3, mysql, postgres, postgresql"},
		{"invalid postgres ssl mode", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080}, Database: DatabaseConfig{Driver: "postgres", Host: "localhost", Database: "test.db", SSLMode: "invalid"}}, true, "database.ssl_mode must be one of: disable, require, verify-ca, verify-full"},
		{"invalid tls min version", Config{App: AppConfig{Name: "app"}, HTTP: HTTPConfig{Port: 8080, TLS: TLSConfig{MinVersion: "1.1"}}, Database: DatabaseConfig{Driver: "sqlite", Database: "test.db"}}, true, "http.tls.min_version must be one of: 1.2, 1.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, tt.config.Validate(), tt.wantErr, tt.errMsg)
		})
	}
}

// TestHTTPConfigGetShutdownTimeout 测试关闭超时计算
func TestHTTPConfigGetShutdownTimeout(t *testing.T) {
	tests := []struct {
		timeout int
		want    time.Duration
	}{
		{0, 30 * time.Second},  // 零值返回默认
		{-1, 30 * time.Second}, // 负值返回默认
		{60, 60 * time.Second}, // 自定义值
		{5, 5 * time.Second},   // 小值
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			c := HTTPConfig{ShutdownTimeout: tt.timeout}
			if got := c.GetShutdownTimeout(); got != tt.want {
				t.Errorf("GetShutdownTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTCPConfigGetShutdownTimeout(t *testing.T) {
	tests := []struct {
		timeout int
		want    time.Duration
	}{
		{0, 30 * time.Second},
		{45, 45 * time.Second},
	}

	for _, tt := range tests {
		c := TCPConfig{ShutdownTimeout: tt.timeout}
		if got := c.GetShutdownTimeout(); got != tt.want {
			t.Errorf("GetShutdownTimeout() = %v, want %v", got, tt.want)
		}
	}
}

func TestUDPConfigGetShutdownTimeout(t *testing.T) {
	tests := []struct {
		timeout int
		want    time.Duration
	}{
		{0, 30 * time.Second},
		{20, 20 * time.Second},
	}

	for _, tt := range tests {
		c := UDPConfig{ShutdownTimeout: tt.timeout}
		if got := c.GetShutdownTimeout(); got != tt.want {
			t.Errorf("GetShutdownTimeout() = %v, want %v", got, tt.want)
		}
	}
}

// TestLoadWithEnvFromTestdata 从 testdata 加载配置
func TestLoadWithEnvFromTestdata(t *testing.T) {
	t.Setenv(EnvVarAppEnv, "")
	tests := []struct {
		name     string
		filename string
		wantErr  bool
		check    func(t *testing.T, cfg *Config)
	}{
		{
			name:     "load base config",
			filename: "config.yaml",
			check: func(t *testing.T, cfg *Config) {
				assertEq(t, "App.Name", cfg.App.Name, "test-app")
				assertEq(t, "HTTP.Port", cfg.HTTP.Port, 9090)
				assertEq(t, "HTTP.Host", cfg.HTTP.Host, "127.0.0.1")
				assertEq(t, "Database.Driver", cfg.Database.Driver, "sqlite")
			},
		},
		{
			name:     "load minimal config uses defaults",
			filename: "minimal.yaml",
			check: func(t *testing.T, cfg *Config) {
				assertEq(t, "App.Name", cfg.App.Name, "minimal-app")
				assertEq(t, "App.Version", cfg.App.Version, "1.0.0") // 默认值
				assertEq(t, "HTTP.Port", cfg.HTTP.Port, 8080)        // 默认值
				assertEq(t, "Database.Driver", cfg.Database.Driver, constant.DriverSQLite)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadWithEnv(filepath.Join(testdataDir(), tt.filename), "")
			assertError(t, err, tt.wantErr, "")
			if tt.check != nil && err == nil {
				tt.check(t, cfg)
			}
		})
	}
}

// TestLoadWithEnvMerge 测试环境配置合并
func TestLoadWithEnvMerge(t *testing.T) {
	baseConfig := filepath.Join(testdataDir(), "config.yaml")

	tests := []struct {
		name        string
		envOverride string
		wantName    string
		wantPort    int
		wantLevel   string
		wantEnv     string
	}{
		{"no override", "", "test-app", 9090, "info", "dev"},
		{"dev override", "dev", "dev-app", 9091, "debug", "dev"},
		{"prod override", "prod", "prod-app", 80, "warn", "prod"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvVarAppEnv, "") // isolate from host APP_ENV
			cfg, meta, err := LoadWithMeta(baseConfig, tt.envOverride)
			if err != nil {
				t.Fatalf("LoadWithMeta() error: %v", err)
			}
			assertEq(t, "App.Name", cfg.App.Name, tt.wantName)
			assertEq(t, "HTTP.Port", cfg.HTTP.Port, tt.wantPort)
			assertEq(t, "Log.Level", cfg.Log.Level, tt.wantLevel)
			assertEq(t, "App.Env", cfg.App.Env, tt.wantEnv)
			if tt.envOverride == "" && meta.OverlayPath != "" {
				t.Errorf("expected no overlay, got %q", meta.OverlayPath)
			}
			if tt.envOverride != "" && meta.OverlayPath == "" {
				t.Error("expected overlay path to be set")
			}
		})
	}
}

func TestLoadWithEnvNoOverlayForNonConfigYAML(t *testing.T) {
	t.Setenv(EnvVarAppEnv, "")
	minimal := filepath.Join(testdataDir(), "minimal.yaml")
	cfg, meta, err := LoadWithMeta(minimal, "dev")
	if err != nil {
		t.Fatalf("LoadWithMeta() error: %v", err)
	}
	assertEq(t, "App.Name", cfg.App.Name, "minimal-app")
	assertEq(t, "HTTP.Port", cfg.HTTP.Port, 8080) // defaults, not neighbor config.dev.yaml (9091)
	assertEq(t, "App.Env", cfg.App.Env, "dev")    // locked by overlayEnv even without merge
	if meta.OverlayPath != "" || meta.OverlayEnv != "" {
		t.Errorf("minimal.yaml must not attempt overlay, path=%q env=%q", meta.OverlayPath, meta.OverlayEnv)
	}
}

func TestLoadStandaloneProdFileKeepsEnv(t *testing.T) {
	t.Setenv(EnvVarAppEnv, "")
	cfg, meta, err := LoadWithMeta(filepath.Join(testdataDir(), "config.prod.yaml"), "")
	if err != nil {
		t.Fatalf("LoadWithMeta() error: %v", err)
	}
	assertEq(t, "App.Env", cfg.App.Env, "prod")
	assertEq(t, "HTTP.Port", cfg.HTTP.Port, 80)
	if meta.OverlayPath != "" {
		t.Errorf("standalone overlay file must not self-merge, got %q", meta.OverlayPath)
	}
}

func TestAPPEnvMergesOverlayWithoutFlag(t *testing.T) {
	t.Setenv(EnvVarAppEnv, "prod")
	cfg, meta, err := LoadWithMeta(filepath.Join(testdataDir(), "config.yaml"), "")
	if err != nil {
		t.Fatalf("LoadWithMeta() error: %v", err)
	}
	assertEq(t, "App.Name", cfg.App.Name, "prod-app")
	assertEq(t, "App.Env", cfg.App.Env, "prod")
	assertEq(t, "HTTP.Port", cfg.HTTP.Port, 80)
	if meta.OverlayPath == "" {
		t.Error("APP_ENV=prod should merge config.prod.yaml")
	}
}

func TestEnvFlagOverridesYAMLAppEnv(t *testing.T) {
	t.Setenv(EnvVarAppEnv, "")
	// Load prod file but force env=test via flag (no overlay merge because basename != config.yaml)
	cfg, err := LoadWithEnv(filepath.Join(testdataDir(), "config.prod.yaml"), "test")
	if err != nil {
		t.Fatalf("LoadWithEnv() error: %v", err)
	}
	assertEq(t, "App.Env", cfg.App.Env, "test")
}

// TestLoadWithEnvInvalid 测试无效配置加载
func TestLoadWithEnvInvalid(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"nonexistent file", "/nonexistent/path/config.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadWithEnv(tt.path, "")
			if err == nil {
				t.Error("expected error but got nil")
			}
		})
	}
}

// TestEnvironmentVariables 测试环境变量绑定
func TestEnvironmentVariables(t *testing.T) {
	t.Setenv(EnvVarAppEnv, "")
	t.Setenv(EnvVarDatabasePassword, "db_secret")
	t.Setenv(EnvVarRedisPassword, "redis_secret")
	t.Setenv(EnvVarJWTSecret, "jwt-secret-key-must-be-32-bytes!!")
	t.Setenv(EnvVarHTTPInternalServiceKey, "internal-key")
	t.Setenv(EnvVarObjectStorageAccessKey, "ak")
	t.Setenv(EnvVarObjectStorageSecretKey, "sk")

	cfg := mustLoad(t, "config.yaml")

	assertEq(t, "Database.Password", cfg.Database.Password, "db_secret")
	assertEq(t, "Redis.Password", cfg.Redis.Password, "redis_secret")
	assertEq(t, "JWT.Secret", cfg.JWT.Secret, "jwt-secret-key-must-be-32-bytes!!")
	assertEq(t, "HTTP.InternalServiceKey", cfg.HTTP.InternalServiceKey, "internal-key")
	assertEq(t, "ObjectStorage.AccessKey", cfg.ObjectStorage.AccessKey, "ak")
	assertEq(t, "ObjectStorage.SecretKey", cfg.ObjectStorage.SecretKey, "sk")
}

func TestMaskedCopyAndYAMLDump(t *testing.T) {
	cfg := &Config{
		App:      AppConfig{Name: "n", Env: "dev"},
		Database: DatabaseConfig{Password: "secret"},
		JWT:      JWTConfig{Secret: "jwt-secret-key-must-be-32-bytes!!"},
	}
	masked := cfg.MaskedCopy()
	assertEq(t, "Database.Password", masked.Database.Password, "******")
	assertEq(t, "JWT.Secret", masked.JWT.Secret, "******")
	assertEq(t, "original Database.Password", cfg.Database.Password, "secret")

	out, err := cfg.MarshalYAMLMasked()
	if err != nil {
		t.Fatalf("MarshalYAMLMasked: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "******") {
		t.Errorf("expected masked secrets in yaml, got:\n%s", s)
	}
	if strings.Contains(s, "jwt-secret-key-must-be-32-bytes!!") {
		t.Error("raw jwt secret must not appear in dump")
	}
}

func TestEnvExampleListsAllSensitiveBinds(t *testing.T) {
	examplePath := filepath.Join("..", "..", "configs", ".env.example")
	data, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	content := string(data)
	for _, b := range SensitiveBinds {
		if !strings.Contains(content, b.EnvVar+"=") {
			t.Errorf(".env.example missing %s=", b.EnvVar)
		}
	}
}

// assertEq 通用断言辅助函数
func assertEq[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}
