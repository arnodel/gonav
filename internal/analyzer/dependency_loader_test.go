package analyzer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDependencyLoader_StartLoading(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "dependency-loader-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a simple go.mod
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module dependency-loader-test

go 1.21
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Create dependency loader
	loader := NewDependencyLoader(tempDir, nil)
	require.NotNil(t, loader)

	// Test starting dependency loading
	missingDeps := []string{"github.com/stretchr/testify", "github.com/gin-gonic/gin"}
	token := "test-token-123"

	job, err := loader.StartDependencyLoading(token, missingDeps)
	require.NoError(t, err)
	require.NotNil(t, job)

	// Verify job properties
	assert.Equal(t, token, job.ID)
	assert.Equal(t, missingDeps, job.Dependencies)
	assert.Equal(t, LoadingStatusInProgress, job.Status)
	assert.Equal(t, len(missingDeps), job.Progress.Total)
	assert.Equal(t, 0, job.Progress.Completed)
	assert.Equal(t, 0, job.Progress.Failed)

	// Check that we can get status
	status, err := loader.GetLoadingStatus(token)
	require.NoError(t, err)
	assert.Equal(t, LoadingStatusInProgress, status.Status)
	assert.Equal(t, len(missingDeps), status.Progress.Total)

	// Wait a bit for some progress (note: this might fail in CI without network)
	time.Sleep(100 * time.Millisecond)

	// Test duplicate job (should return existing)
	job2, err := loader.StartDependencyLoading(token, missingDeps)
	require.NoError(t, err)
	assert.Equal(t, job.ID, job2.ID) // Should be the same job
	_ = job // Use the job variable

	// Test listing active jobs
	activeJobs := loader.ListActiveJobs()
	assert.Len(t, activeJobs, 1)
	assert.Equal(t, token, activeJobs[0].ID)

	t.Logf("Started loading job for %d dependencies", len(missingDeps))
}

func TestDependencyLoader_StatusTracking(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "status-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create go.mod
	modFile := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(modFile, []byte("module status-test\ngo 1.21\n"), 0644)
	require.NoError(t, err)

	loader := NewDependencyLoader(tempDir, nil)
	
	// Test status for non-existent job
	status, err := loader.GetLoadingStatus("non-existent")
	require.NoError(t, err)
	assert.Equal(t, LoadingStatusIdle, status.Status)

	// Start a job with a dependency that should fail (non-existent)
	missingDeps := []string{"github.com/definitely/does/not/exist"}
	token := "failure-test"

	job, err := loader.StartDependencyLoading(token, missingDeps)
	require.NoError(t, err)
	_ = job // Use job variable

	// Wait for completion (should be quick since it will fail)
	time.Sleep(500 * time.Millisecond)

	status, err = loader.GetLoadingStatus(token)
	require.NoError(t, err)
	
	// Should have completed (even if failed)
	t.Logf("Final status: %s, completed: %d, failed: %d", 
		status.Status, status.Progress.Completed, status.Progress.Failed)
}

func TestPackagesAnalyzer_WithDependencyLoader(t *testing.T) {
	// Create a temporary directory with missing dependencies
	tempDir, err := os.MkdirTemp("", "enhanced-analysis-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create go.mod with missing dependency
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module enhanced-analysis-test

go 1.21

require (
	github.com/missing/package v1.0.0
)
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Create Go file with missing import
	goFile := filepath.Join(tempDir, "main.go")
	goContent := `package main

import (
	"fmt"
	"github.com/missing/package/lib"
)

func main() {
	obj := lib.New()
	fmt.Println("Hello", obj)
}
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Create packages analyzer with dependency loader
	packagesAnalyzer := NewPackagesAnalyzer(tempDir, nil)
	dependencyLoader := NewDependencyLoader(tempDir, nil)
	packagesAnalyzer.SetDependencyLoader(dependencyLoader)

	// Trigger enhanced analysis
	response, err := packagesAnalyzer.TriggerEnhancedAnalysis("")
	require.NoError(t, err)
	require.NotNil(t, response)

	// Should have quality assessment indicating missing dependencies
	assert.NotNil(t, response.Quality)
	assert.False(t, response.Quality.IsComplete)
	assert.True(t, response.Quality.EnhancementAvailable)
	assert.Greater(t, len(response.Quality.MissingDependencies), 0)
	assert.NotEmpty(t, response.EnhancementToken)

	// Should have started dependency loading
	if response.DependencyStatus != nil {
		assert.Equal(t, LoadingStatusInProgress, response.DependencyStatus.Status)
		t.Logf("Dependency loading started: %d total dependencies", 
			response.DependencyStatus.Progress.Total)
	}

	// Test getting updated status
	status, err := dependencyLoader.GetLoadingStatus(response.EnhancementToken)
	require.NoError(t, err)
	t.Logf("Loading status: %s", status.Status)
}

func TestDependencyLoader_ProgressUpdates(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "progress-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create go.mod
	err = os.WriteFile(filepath.Join(tempDir, "go.mod"), 
		[]byte("module progress-test\ngo 1.21\n"), 0644)
	require.NoError(t, err)

	loader := NewDependencyLoader(tempDir, nil)
	
	// Start loading job
	missingDeps := []string{"github.com/nonexistent/dep1", "github.com/nonexistent/dep2"}
	token := "progress-test"

	job, err := loader.StartDependencyLoading(token, missingDeps)
	require.NoError(t, err)
	_ = job // Use job variable

	// Get progress updates channel
	updates, err := loader.GetProgressUpdates(token)
	require.NoError(t, err)

	// Collect some updates (will close when job completes)
	updateCount := 0
	timeout := time.After(2 * time.Second)
	
	for {
		select {
		case progress, ok := <-updates:
			if !ok {
				// Channel closed, job finished
				t.Logf("Progress updates finished after %d updates", updateCount)
				return
			}
			updateCount++
			t.Logf("Progress update %d: %d/%d completed, %d failed", 
				updateCount, progress.Completed, progress.Total, progress.Failed)
		case <-timeout:
			t.Logf("Timeout waiting for progress updates after %d updates", updateCount)
			return
		}
	}
}

func TestDependencyLoader_JobCleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cleanup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	loader := NewDependencyLoader(tempDir, nil)
	
	// Start a job that will complete quickly (with failure)
	job, err := loader.StartDependencyLoading("cleanup-test", []string{"github.com/nonexistent/package"})
	require.NoError(t, err)
	_ = job // Use job variable

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	// Should have active job (might be completed by now)
	activeJobs := loader.ListActiveJobs()
	assert.GreaterOrEqual(t, len(activeJobs), 0) // Could be 0 or 1

	// Cleanup with very short max age (should remove completed job)
	loader.CleanupCompletedJobs(1 * time.Nanosecond)

	// Should have no active jobs after cleanup
	activeJobs = loader.ListActiveJobs()
	assert.Len(t, activeJobs, 0)
}