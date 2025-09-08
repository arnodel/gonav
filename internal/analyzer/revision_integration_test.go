package analyzer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevisionAnalyzer_CompleteFlow(t *testing.T) {
	// Create a temporary directory with a Go package that has missing dependencies
	tempDir, err := os.MkdirTemp("", "revision-flow-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create go.mod with missing dependency
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module revision-flow-test

go 1.21

require (
	github.com/nonexistent/missing-package v1.0.0
)
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Create Go file with missing import
	goFile := filepath.Join(tempDir, "main.go")
	goContent := `package main

import (
	"fmt"
	"github.com/nonexistent/missing-package/lib"
)

func main() {
	obj := lib.New()
	fmt.Println("Hello", obj)
}

var GlobalVar = "test"

type TestStruct struct {
	Field string
}

func (t TestStruct) Method() string {
	return t.Field
}
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Create revision analyzer with small queue for testing
	queueConfig := DependencyQueueConfig{
		MaxConcurrentDownloads: 2,
		DownloadTimeout:        30 * time.Second,
		QueueSize:             10,
		RetryAttempts:         1,
	}
	
	analyzer := NewRevisionAnalyzer(tempDir, nil, queueConfig)
	require.NotNil(t, analyzer)

	// Test 1: Initial package request (no revision)
	t.Run("InitialPackageRequest", func(t *testing.T) {
		response, err := analyzer.AnalyzePackage("", "")
		require.NoError(t, err)
		require.NotNil(t, response)

		// Should have analysis results
		assert.NotNil(t, response.PackageInfo)
		assert.Equal(t, "main", response.PackageInfo.Name)
		assert.Greater(t, len(response.PackageInfo.Symbols), 0)

		// Should have revision and be incomplete
		assert.NotEmpty(t, response.Revision)
		assert.False(t, response.Complete) // Has missing dependencies
		assert.False(t, response.NoChange)
		
		// Quality information should indicate missing dependencies
		assert.NotNil(t, response.Quality)
		assert.Greater(t, len(response.Quality.MissingDependencies), 0)

		t.Logf("Initial package analysis: revision=%s, complete=%v, symbols=%d", 
			response.Revision, response.Complete, len(response.PackageInfo.Symbols))
	})

	// Test 2: Second request with same revision (should return no change)
	t.Run("SameRevisionRequest", func(t *testing.T) {
		// First get initial response
		initialResponse, err := analyzer.AnalyzePackage("", "")
		require.NoError(t, err)

		// Request again with same revision
		response, err := analyzer.AnalyzePackage("", initialResponse.Revision)
		require.NoError(t, err)

		// Should return no change
		assert.Equal(t, initialResponse.Revision, response.Revision)
		assert.True(t, response.NoChange)
		assert.Nil(t, response.PackageInfo) // No data when no change
	})

	// Test 3: File analysis with revision tracking
	t.Run("FileAnalysisWithRevision", func(t *testing.T) {
		response, err := analyzer.AnalyzeFile("", "main.go", "")
		require.NoError(t, err)
		require.NotNil(t, response)

		// Should have file analysis results
		assert.NotNil(t, response.FileInfo)
		assert.Greater(t, len(response.FileInfo.Source), 0)
		assert.Greater(t, len(response.FileInfo.Symbols), 0)
		assert.Greater(t, len(response.FileInfo.References), 0)

		// Should have different revision from package analysis
		assert.NotEmpty(t, response.Revision)
		assert.False(t, response.Complete) // Has missing dependencies

		t.Logf("File analysis: revision=%s, complete=%v, symbols=%d, refs=%d", 
			response.Revision, response.Complete, 
			len(response.FileInfo.Symbols), len(response.FileInfo.References))

		// Test same revision request for file
		sameRevisionResponse, err := analyzer.AnalyzeFile("", "main.go", response.Revision)
		require.NoError(t, err)
		assert.True(t, sameRevisionResponse.NoChange)
	})

	// Test 4: Cache statistics
	t.Run("CacheStatistics", func(t *testing.T) {
		stats := analyzer.GetCacheStats()
		assert.Greater(t, stats.TotalEntries, 0)
		assert.Greater(t, stats.IncompleteEntries, 0) // Should have incomplete entries
		
		queueStats := analyzer.GetQueueStats()
		t.Logf("Cache stats: total=%d, incomplete=%d, loading=%d", 
			stats.TotalEntries, stats.IncompleteEntries, stats.LoadingInProgress)
		t.Logf("Queue stats: total_requests=%d, active=%d", 
			queueStats.TotalRequests, queueStats.ActiveDownloads)
	})

	// Test 5: Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		// Cleanup old entries (none should be removed since they're recent)
		analyzer.Cleanup(1 * time.Hour)
		
		stats := analyzer.GetCacheStats()
		assert.Greater(t, stats.TotalEntries, 0) // Should still have entries
	})

	// Shutdown the analyzer
	err = analyzer.Shutdown(5 * time.Second)
	assert.NoError(t, err)
}

func TestRevisionAnalyzer_CompletedAnalysis(t *testing.T) {
	// Create a package with no external dependencies (should be complete)
	tempDir, err := os.MkdirTemp("", "complete-analysis-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create go.mod without external dependencies
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module complete-analysis-test

go 1.21
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Create Go file with only stdlib imports
	goFile := filepath.Join(tempDir, "complete.go")
	goContent := `package main

import (
	"fmt"
	"os"
)

func main() {
	cwd, _ := os.Getwd()
	fmt.Printf("Working in: %s\n", cwd)
}

const Version = "1.0.0"
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Create analyzer
	queueConfig := DefaultDependencyQueueConfig()
	analyzer := NewRevisionAnalyzer(tempDir, nil, queueConfig)

	// Analyze package
	response, err := analyzer.AnalyzePackage("", "")
	require.NoError(t, err)
	require.NotNil(t, response)

	// Should be complete analysis
	assert.NotNil(t, response.PackageInfo)
	assert.NotEmpty(t, response.Revision)
	assert.True(t, response.Complete) // No missing dependencies
	assert.False(t, response.NoChange)
	
	// Quality should indicate complete analysis
	assert.NotNil(t, response.Quality)
	assert.True(t, response.Quality.IsComplete)
	assert.Len(t, response.Quality.MissingDependencies, 0)
	assert.Equal(t, 1.0, response.Quality.QualityScore)

	t.Logf("Complete analysis: revision=%s, complete=%v, score=%.2f", 
		response.Revision, response.Complete, response.Quality.QualityScore)

	// Second request should return same complete result
	response2, err := analyzer.AnalyzePackage("", "")
	require.NoError(t, err)
	assert.Equal(t, response.Revision, response2.Revision)
	assert.True(t, response2.Complete)

	// Cleanup
	err = analyzer.Shutdown(2 * time.Second)
	assert.NoError(t, err)
}

func TestRevisionGeneration_Consistency(t *testing.T) {
	// Test that revision generation is consistent
	quality1 := &AnalysisQuality{
		IsComplete:          false,
		AnalysisMode:        AnalysisModePartial,
		QualityScore:        0.75,
		MissingDependencies: []string{"github.com/gin-gonic/gin", "github.com/lib/pq"},
	}

	quality2 := &AnalysisQuality{
		IsComplete:          false,
		AnalysisMode:        AnalysisModePartial,
		QualityScore:        0.75,
		MissingDependencies: []string{"github.com/lib/pq", "github.com/gin-gonic/gin"}, // Different order
	}

	quality3 := &AnalysisQuality{
		IsComplete:          true,
		AnalysisMode:        AnalysisModeComplete,
		QualityScore:        1.0,
		MissingDependencies: []string{},
	}

	revision1 := GenerateRevision("test-package", quality1, 10, 50)
	revision2 := GenerateRevision("test-package", quality2, 10, 50)
	revision3 := GenerateRevision("test-package", quality3, 15, 75) // Different counts

	// Same quality should produce same revision (even with different order of dependencies)
	assert.Equal(t, revision1, revision2, "Revisions should be the same for equivalent quality")

	// Different quality should produce different revision
	assert.NotEqual(t, revision1, revision3, "Revisions should be different for different quality")

	t.Logf("Revision 1 (partial): %s", revision1)
	t.Logf("Revision 2 (partial, reordered): %s", revision2)
	t.Logf("Revision 3 (complete): %s", revision3)
}