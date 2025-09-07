package env

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIsolated(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gonav-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	env, err := NewIsolated(tempDir)
	require.NoError(t, err)

	// Verify directories are created
	assert.DirExists(t, env.GoModCache)
	assert.DirExists(t, env.GoCache)
	assert.DirExists(t, env.GoPath)

	// Verify environment variables are set
	envVars := env.Environment()
	expectedVars := []string{
		fmt.Sprintf("GOMODCACHE=%s", env.GoModCache),
		fmt.Sprintf("GOCACHE=%s", env.GoCache),
		fmt.Sprintf("GOPATH=%s", env.GoPath),
		"GO111MODULE=on",
	}

	for _, expected := range expectedVars {
		assert.Contains(t, envVars, expected)
	}
}

func TestIsolatedEnv_ExecCommand(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gonav-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	env, err := NewIsolated(tempDir)
	require.NoError(t, err)

	// Test that commands use isolated environment
	cmd := env.ExecCommand("go", "env", "GOMODCACHE")
	output, err := cmd.Output()
	require.NoError(t, err)

	// Should return our isolated GOMODCACHE, not the host one
	assert.Contains(t, string(output), env.GoModCache)
}

func TestIsolatedEnv_DownloadModule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gonav-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	env, err := NewIsolated(tempDir)
	require.NoError(t, err)

	// Test downloading a known module
	downloadInfo, err := env.DownloadModule("github.com/arnodel/golua@v0.1.0")
	require.NoError(t, err)

	// Verify download info
	assert.Equal(t, "github.com/arnodel/golua", downloadInfo.Path)
	assert.Equal(t, "v0.1.0", downloadInfo.Version)
	assert.NotEmpty(t, downloadInfo.Dir)

	// Verify module was downloaded to isolated cache
	assert.Contains(t, downloadInfo.Dir, env.GoModCache)
	assert.DirExists(t, downloadInfo.Dir)

	// Verify module directory contains expected files
	entries, err := os.ReadDir(downloadInfo.Dir)
	require.NoError(t, err)

	hasGoMod := false
	hasGoFiles := false
	for _, entry := range entries {
		if entry.Name() == "go.mod" {
			hasGoMod = true
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			hasGoFiles = true
		}
	}

	assert.True(t, hasGoMod, "Module should contain go.mod file")
	assert.True(t, hasGoFiles, "Module should contain .go files")
}

func TestIsolatedEnv_ModuleCachePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gonav-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	env, err := NewIsolated(tempDir)
	require.NoError(t, err)

	path := env.ModuleCachePath("github.com/example/module", "v1.0.0")
	expected := filepath.Join(env.GoModCache, "github.com/example/module@v1.0.0")
	assert.Equal(t, expected, path)
}

func TestIsolatedEnv_Stats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gonav-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	env, err := NewIsolated(tempDir)
	require.NoError(t, err)

	// Initial stats
	stats := env.Stats()
	assert.Equal(t, tempDir, stats["base_dir"])
	assert.Equal(t, env.GoModCache, stats["gomodcache"])
	assert.Equal(t, 0, stats["cached_modules"])

	// Download a module and check stats again
	_, err = env.DownloadModule("github.com/arnodel/golua@v0.1.0")
	require.NoError(t, err)

	stats = env.Stats()
	assert.Equal(t, 1, stats["cached_modules"])
}

func TestIsolatedEnv_Cleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gonav-test-*")
	require.NoError(t, err)

	env, err := NewIsolated(tempDir)
	require.NoError(t, err)

	// Verify directory exists
	assert.DirExists(t, tempDir)

	// Cleanup should remove the directory
	err = env.Cleanup()
	require.NoError(t, err)

	// Directory should no longer exist
	_, err = os.Stat(tempDir)
	assert.True(t, os.IsNotExist(err))
}

func TestIsolatedEnv_HostIsolation(t *testing.T) {
	// This test verifies that the isolated environment doesn't affect the host
	tempDir, err := os.MkdirTemp("", "gonav-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Get original host GOMODCACHE
	hostCmd := exec.Command("go", "env", "GOMODCACHE")
	hostOutput, err := hostCmd.Output()
	require.NoError(t, err)
	hostGoModCache := strings.TrimSpace(string(hostOutput))

	// Create isolated environment
	env, err := NewIsolated(tempDir)
	require.NoError(t, err)

	// Download module in isolation
	_, err = env.DownloadModule("github.com/arnodel/golua@v0.1.0")
	require.NoError(t, err)

	// Verify host GOMODCACHE is unchanged
	hostCmd2 := exec.Command("go", "env", "GOMODCACHE")
	hostOutput2, err := hostCmd2.Output()
	require.NoError(t, err)
	hostGoModCache2 := strings.TrimSpace(string(hostOutput2))

	assert.Equal(t, hostGoModCache, hostGoModCache2, "Host GOMODCACHE should be unchanged")

	// Verify the isolated cache path is different from host
	assert.NotEqual(t, hostGoModCache, env.GoModCache, "Isolated cache should be different from host")
}

func TestIsolatedEnv_ErrorHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gonav-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	env, err := NewIsolated(tempDir)
	require.NoError(t, err)

	// Test downloading non-existent module
	_, err = env.DownloadModule("github.com/nonexistent/fake-module@v1.0.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "go mod download failed")
}

// Benchmark isolation performance
func BenchmarkIsolatedDownload(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "gonav-bench-*")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	env, err := NewIsolated(tempDir)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Download the same module repeatedly (should be cached after first time)
		_, err := env.DownloadModule("github.com/arnodel/golua@v0.1.0")
		require.NoError(b, err)
	}
}