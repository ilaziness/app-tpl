package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetConfigCLIFlags(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "")
	t.Setenv("DATABASE_PASSWORD", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("HTTP_INTERNAL_SERVICE_KEY", "")
	t.Setenv("OBJECT_STORAGE_ACCESS_KEY", "")
	t.Setenv("OBJECT_STORAGE_SECRET_KEY", "")
	cfgFile = ""
	env = ""
	configShowCheck = false
}

func TestExecuteE_configValidate_validConfig(t *testing.T) {
	resetConfigCLIFlags(t)
	base := filepath.Join("..", "internal", "config", "testdata", "config.yaml")

	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetArgs([]string{"config", "validate", "-c", base})

	err := ExecuteE()
	require.NoError(t, err)
	out := stdout.String()
	assert.Contains(t, out, "Configuration is valid")
	assert.Contains(t, out, "Environment: dev")
	assert.Contains(t, out, "Base config:")
	assert.NotContains(t, out, "Overlay (")
}

func TestExecuteE_configValidate_withOverlay(t *testing.T) {
	resetConfigCLIFlags(t)
	base := filepath.Join("..", "internal", "config", "testdata", "config.yaml")

	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetArgs([]string{"config", "validate", "-c", base, "-e", "dev"})

	err := ExecuteE()
	require.NoError(t, err)
	out := stdout.String()
	assert.Contains(t, out, "Configuration is valid")
	assert.Contains(t, out, "Environment: dev")
	assert.Contains(t, out, "config.dev.yaml")
}

func TestExecuteE_configValidate_missingConfig(t *testing.T) {
	resetConfigCLIFlags(t)
	missing := filepath.Join("..", "internal", "config", "testdata", "does-not-exist.yaml")

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetArgs([]string{"config", "validate", "-c", missing})

	err := ExecuteE()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "configuration validation failed")
}

func TestExecuteE_configShow_masksSecrets(t *testing.T) {
	resetConfigCLIFlags(t)
	t.Setenv("DATABASE_PASSWORD", "super-secret-db-password")
	t.Setenv("JWT_SECRET", "jwt-secret-key-must-be-32-bytes!!")

	base := filepath.Join("..", "internal", "config", "testdata", "config.yaml")

	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetArgs([]string{"config", "show", "-c", base})

	err := ExecuteE()
	require.NoError(t, err)
	out := stdout.String()
	assert.Contains(t, out, "Base config:")
	assert.NotContains(t, out, "Overlay (")
	assert.Contains(t, out, "******")
	assert.NotContains(t, out, "super-secret-db-password")
	assert.NotContains(t, out, "jwt-secret-key-must-be-32-bytes!!")
	assert.Contains(t, out, "test-app")
}
