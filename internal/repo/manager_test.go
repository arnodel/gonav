package repo

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	// Test normal manager creation
	manager, err := NewManager()
	require.NoError(t, err)
	assert.False(t, manager.IsIsolated())
	assert.Nil(t, manager.GetIsolatedEnv())
}

func TestNewManagerWithIsolation(t *testing.T) {
	// Test manager with isolation enabled
	manager, err := NewManager(WithIsolation(true))
	require.NoError(t, err)
	defer manager.Cleanup()

	assert.True(t, manager.IsIsolated())
	assert.NotNil(t, manager.GetIsolatedEnv())

	// Verify isolation environment is properly set up
	env := manager.GetIsolatedEnv()
	assert.DirExists(t, env.GoModCache)
	assert.DirExists(t, env.GoCache)
	assert.DirExists(t, env.GoPath)
}

func TestNewManagerWithIsolationDisabled(t *testing.T) {
	// Test manager with isolation explicitly disabled
	manager, err := NewManager(WithIsolation(false))
	require.NoError(t, err)

	assert.False(t, manager.IsIsolated())
	assert.Nil(t, manager.GetIsolatedEnv())
}

func TestManagerStats(t *testing.T) {
	// Test normal manager stats
	manager, err := NewManager()
	require.NoError(t, err)

	stats := manager.Stats()
	assert.Contains(t, stats, "cache_dir")
	assert.Contains(t, stats, "loaded_repositories")
	assert.Contains(t, stats, "isolated")
	assert.Equal(t, false, stats["isolated"])
	assert.Equal(t, 0, stats["loaded_repositories"])

	// Test isolated manager stats
	isolatedManager, err := NewManager(WithIsolation(true))
	require.NoError(t, err)
	defer isolatedManager.Cleanup()

	isolatedStats := isolatedManager.Stats()
	assert.Equal(t, true, isolatedStats["isolated"])
	assert.Contains(t, isolatedStats, "isolation_stats")
}

func TestManagerLoadRepositoryNormal(t *testing.T) {
	manager, err := NewManager()
	require.NoError(t, err)

	// Test loading a repository (this will use host environment)
	repoInfo, err := manager.LoadRepository("github.com/arnodel/golua@v0.1.0")
	require.NoError(t, err)

	assert.Equal(t, "github.com/arnodel/golua@v0.1.0", repoInfo.ModuleAtVersion)
	assert.Equal(t, "github.com/arnodel/golua", repoInfo.ModulePath)
	assert.Equal(t, "v0.1.0", repoInfo.Version)
	assert.Greater(t, len(repoInfo.Files), 0)

	// Verify it's cached
	stats := manager.Stats()
	assert.Equal(t, 1, stats["loaded_repositories"])
}

func TestManagerLoadRepositoryIsolated(t *testing.T) {
	manager, err := NewManager(WithIsolation(true))
	require.NoError(t, err)
	defer manager.Cleanup()

	// Test loading a repository in isolation
	repoInfo, err := manager.LoadRepository("github.com/arnodel/golua@v0.1.0")
	require.NoError(t, err)

	assert.Equal(t, "github.com/arnodel/golua@v0.1.0", repoInfo.ModuleAtVersion)
	assert.Equal(t, "github.com/arnodel/golua", repoInfo.ModulePath)
	assert.Equal(t, "v0.1.0", repoInfo.Version)
	assert.Greater(t, len(repoInfo.Files), 0)

	// Verify isolation stats show the module was downloaded
	stats := manager.Stats()
	isolationStats := stats["isolation_stats"].(map[string]interface{})
	assert.Equal(t, 1, isolationStats["cached_modules"])
}

func TestManagerHostIsolation(t *testing.T) {
	// This test verifies that isolated downloads don't affect host
	// Get host GOMODCACHE before test
	hostEnv := os.Getenv("GOMODCACHE")

	manager, err := NewManager(WithIsolation(true))
	require.NoError(t, err)
	defer manager.Cleanup()

	// Download in isolation
	_, err = manager.LoadRepository("github.com/arnodel/golua@v0.1.0")
	require.NoError(t, err)

	// Verify host environment is unchanged
	hostEnvAfter := os.Getenv("GOMODCACHE")
	assert.Equal(t, hostEnv, hostEnvAfter, "Host GOMODCACHE should be unchanged")
}

func TestManagerCleanup(t *testing.T) {
	manager, err := NewManager(WithIsolation(true))
	require.NoError(t, err)

	// Get the isolation directory path before cleanup
	env := manager.GetIsolatedEnv()
	isolationDir := env.BaseDir

	// Verify directory exists
	assert.DirExists(t, isolationDir)

	// Cleanup should remove the isolation directory
	err = manager.Cleanup()
	require.NoError(t, err)

	// Directory should no longer exist
	_, err = os.Stat(isolationDir)
	assert.True(t, os.IsNotExist(err))
}

func TestManagerCompatibility(t *testing.T) {
	// Test that both normal and isolated managers produce same repo info structure
	normalManager, err := NewManager()
	require.NoError(t, err)

	isolatedManager, err := NewManager(WithIsolation(true))
	require.NoError(t, err)
	defer isolatedManager.Cleanup()

	// Load same repository with both managers
	normalRepo, err1 := normalManager.LoadRepository("github.com/arnodel/golua@v0.1.0")
	isolatedRepo, err2 := isolatedManager.LoadRepository("github.com/arnodel/golua@v0.1.0")

	require.NoError(t, err1)
	require.NoError(t, err2)

	// Should produce compatible results
	assert.Equal(t, normalRepo.ModuleAtVersion, isolatedRepo.ModuleAtVersion)
	assert.Equal(t, normalRepo.ModulePath, isolatedRepo.ModulePath)
	assert.Equal(t, normalRepo.Version, isolatedRepo.Version)
	
	// Both should have Go files
	assert.Greater(t, len(normalRepo.Files), 0)
	assert.Greater(t, len(isolatedRepo.Files), 0)
}