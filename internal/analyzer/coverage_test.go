package analyzer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComprehensiveCoverage provides comprehensive test coverage for all components
func TestComprehensiveCoverage(t *testing.T) {
	// This test ensures we have coverage for all the major code paths
	// in our revision-based progressive enhancement system
	
	tempDir, err := os.MkdirTemp("", "coverage-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	
	// Setup test environment
	setupTestEnvironment(t, tempDir)
	
	// Test all major components
	t.Run("AnalysisCache_ComprehensiveCoverage", testAnalysisCacheCoverage)
	t.Run("DependencyQueue_ComprehensiveCoverage", testDependencyQueueCoverage) 
	t.Run("RevisionSystem_ComprehensiveCoverage", testRevisionSystemCoverage)
	t.Run("RevisionAnalyzer_ComprehensiveCoverage", func(t *testing.T) {
		testRevisionAnalyzerCoverage(t, tempDir)
	})
	t.Run("QualityAssessment_ComprehensiveCoverage", testQualityAssessmentCoverage)
	t.Run("ErrorHandling_ComprehensiveCoverage", testErrorHandlingCoverage)
}

func setupTestEnvironment(t *testing.T, tempDir string) {
	// Create complete test environment with various scenarios
	
	// go.mod with mixed dependencies
	modContent := `module coverage-test

go 1.21

require (
	github.com/stretchr/testify v1.8.0
	github.com/nonexistent/package v1.0.0
)
`
	err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(modContent), 0644)
	require.NoError(t, err)
	
	// Main file with various constructs
	mainContent := `package main

import (
	"fmt"
	"os"
	"github.com/stretchr/testify/assert"
	"github.com/nonexistent/package/missing"
)

func main() {
	fmt.Println("Coverage test")
}

var GlobalVar = "test"
const GlobalConst = 42

type TestStruct struct {
	Field string
}

func (t *TestStruct) Method() string {
	return t.Field
}

func (t TestStruct) ValueMethod() int {
	return len(t.Field)
}

type TestInterface interface {
	Method() string
}

func FunctionWithDependencies() {
	// Uses both available and missing dependencies
	assert.True(nil, true) // Available dependency
	obj := missing.New()   // Missing dependency
	fmt.Println(obj)
}
`
	err = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)
	
	// Subpackage
	subDir := filepath.Join(tempDir, "sub")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)
	
	subContent := `package sub

import "fmt"

type SubType struct {
	Value int
}

func (s SubType) String() string {
	return fmt.Sprintf("SubType{%d}", s.Value)
}
`
	err = os.WriteFile(filepath.Join(subDir, "sub.go"), []byte(subContent), 0644)
	require.NoError(t, err)
}

func testAnalysisCacheCoverage(t *testing.T) {
	// Test all cache functionality
	dependencyChecker := &SimpleDependencyChecker{}
	cache := NewAnalysisCache(dependencyChecker)
	
	// Test cache key creation and string conversion
	packageKey := CacheKey{Type: CacheKeyTypePackage, PackagePath: "test-pkg"}
	fileKey := CacheKey{Type: CacheKeyTypeFile, PackagePath: "test-pkg", FilePath: "test.go"}
	
	assert.Equal(t, "package:test-pkg", packageKey.String())
	assert.Equal(t, "file:test-pkg:test.go", fileKey.String())
	
	// Test cache miss
	cached, result := cache.Get(packageKey, "")
	assert.Nil(t, cached)
	assert.Equal(t, CacheResultMiss, result)
	
	// Test cache set and hit
	analysis := &CachedAnalysis{
		Revision:            "rev1",
		Quality:             &AnalysisQuality{IsComplete: false},
		Timestamp:           time.Now(),
		MissingDependencies: []string{"dep1", "dep2"},
		IsComplete:          false,
	}
	
	cache.Set(packageKey, analysis)
	
	// Test cache hit
	cached, result = cache.Get(packageKey, "")
	assert.NotNil(t, cached)
	assert.Equal(t, CacheResultHit, result)
	assert.Equal(t, "rev1", cached.Revision)
	
	// Test no change
	cached, result = cache.Get(packageKey, "rev1")
	assert.NotNil(t, cached)
	assert.Equal(t, CacheResultNoChange, result)
	
	// Test newer revision
	newAnalysis := &CachedAnalysis{
		Revision:   "rev2",
		Quality:    &AnalysisQuality{IsComplete: true},
		Timestamp:  time.Now(),
		IsComplete: true,
	}
	cache.Set(packageKey, newAnalysis)
	
	cached, result = cache.Get(packageKey, "rev1")
	assert.NotNil(t, cached)
	assert.Equal(t, CacheResultNewer, result)
	assert.Equal(t, "rev2", cached.Revision)
	
	// Test recalculation logic
	shouldRecalc, availableDeps, err := cache.ShouldRecalculate(packageKey, "/tmp")
	assert.NoError(t, err)
	assert.False(t, shouldRecalc) // Complete analysis shouldn't recalculate
	assert.Len(t, availableDeps, 0)
	
	// Test dependency loading marking
	cache.MarkDependencyLoadingInProgress(packageKey, true)
	cached, _ = cache.Get(packageKey, "")
	assert.True(t, cached.DependencyLoadingInProgress)
	
	cache.MarkDependencyLoadingInProgress(packageKey, false)
	cached, _ = cache.Get(packageKey, "")
	assert.False(t, cached.DependencyLoadingInProgress)
	
	// Test stats
	stats := cache.GetStats()
	assert.Greater(t, stats.TotalEntries, 0)
	
	// Test cleanup
	removed := cache.Cleanup(1 * time.Nanosecond) // Remove all incomplete entries
	assert.Equal(t, 0, removed) // Complete entry should remain
}

func testDependencyQueueCoverage(t *testing.T) {
	// Test queue configuration
	config := DefaultDependencyQueueConfig()
	assert.Equal(t, 3, config.MaxConcurrentDownloads)
	assert.Equal(t, 2*time.Minute, config.DownloadTimeout)
	
	// Custom config
	customConfig := DependencyQueueConfig{
		MaxConcurrentDownloads: 1,
		DownloadTimeout:        10 * time.Second,
		QueueSize:             5,
		RetryAttempts:         1,
	}
	
	queue := NewDependencyQueue(customConfig)
	assert.NotNil(t, queue)
	
	// Test stats before any requests
	stats := queue.GetStats()
	assert.Equal(t, int64(0), stats.TotalRequests)
	
	// Test request submission
	resultChan := make(chan DependencyDownloadResult, 1)
	req := DependencyDownloadRequest{
		WorkDir:      "/tmp",
		Dependencies: []string{"github.com/nonexistent/dep"},
		CacheKey:     CacheKey{Type: CacheKeyTypePackage, PackagePath: "test"},
		RequestID:    "test-req-1",
		ResultChan:   resultChan,
	}
	
	err := queue.SubmitDownloadRequest(req)
	assert.NoError(t, err)
	
	// Test duplicate request
	err = queue.SubmitDownloadRequest(req)
	assert.Error(t, err) // Should fail due to duplicate
	
	// Test active check
	isActive := queue.IsActive(req.CacheKey)
	assert.True(t, isActive)
	
	// Wait for result
	select {
	case result := <-resultChan:
		assert.Equal(t, req.RequestID, result.RequestID)
		assert.Greater(t, len(result.Failed), 0) // Non-existent dep should fail
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for dependency result")
	}
	
	// Test stats after request
	stats = queue.GetStats()
	assert.Greater(t, stats.TotalRequests, int64(0))
	
	// Test shutdown
	err = queue.Shutdown(2 * time.Second)
	assert.NoError(t, err)
}

func testRevisionSystemCoverage(t *testing.T) {
	// Test revision generation
	quality1 := &AnalysisQuality{
		IsComplete:          false,
		AnalysisMode:        AnalysisModePartial,
		QualityScore:        0.75,
		MissingDependencies: []string{"dep1", "dep2"},
	}
	
	quality2 := &AnalysisQuality{
		IsComplete:          true,
		AnalysisMode:        AnalysisModeComplete,
		QualityScore:        1.0,
		MissingDependencies: []string{},
	}
	
	rev1 := GenerateRevision("test-pkg", quality1, 10, 50)
	rev2 := GenerateRevision("test-pkg", quality2, 10, 50)
	
	assert.NotEmpty(t, rev1)
	assert.NotEmpty(t, rev2)
	assert.NotEqual(t, rev1, rev2)
	
	// Test revision consistency
	rev1Again := GenerateRevision("test-pkg", quality1, 10, 50)
	assert.Equal(t, rev1, rev1Again)
	
	// Test revision comparison
	different := CompareRevisions(rev1, rev2)
	assert.True(t, different)
	
	same := CompareRevisions(rev1, rev1)
	assert.False(t, same)
	
	// Test revision info creation
	revInfo := CreateRevisionInfo(rev1, quality1)
	assert.Equal(t, rev1, revInfo.Revision)
	assert.False(t, revInfo.Complete)
	assert.Equal(t, 0.75, revInfo.Quality)
	assert.False(t, revInfo.NoChange)
}

func testRevisionAnalyzerCoverage(t *testing.T, tempDir string) {
	// Test complete revision analyzer functionality
	config := DependencyQueueConfig{
		MaxConcurrentDownloads: 1,
		DownloadTimeout:        5 * time.Second,
		QueueSize:             10,
		RetryAttempts:         1,
	}
	
	analyzer := NewRevisionAnalyzer(tempDir, nil, config)
	require.NotNil(t, analyzer)
	
	// Test package analysis
	packageResp, err := analyzer.AnalyzePackage("", "")
	require.NoError(t, err)
	assert.NotNil(t, packageResp.PackageInfo)
	assert.NotEmpty(t, packageResp.Revision)
	assert.False(t, packageResp.Complete) // Has missing deps
	
	// Test same revision request
	sameRevResp, err := analyzer.AnalyzePackage("", packageResp.Revision)
	require.NoError(t, err)
	assert.True(t, sameRevResp.NoChange)
	assert.Equal(t, packageResp.Revision, sameRevResp.Revision)
	
	// Test file analysis
	fileResp, err := analyzer.AnalyzeFile("", "main.go", "")
	require.NoError(t, err)
	assert.NotNil(t, fileResp.FileInfo)
	assert.NotEmpty(t, fileResp.Revision)
	assert.False(t, fileResp.Complete)
	
	// Test file same revision
	sameFileResp, err := analyzer.AnalyzeFile("", "main.go", fileResp.Revision)
	require.NoError(t, err)
	assert.True(t, sameFileResp.NoChange)
	
	// Test stats
	cacheStats := analyzer.GetCacheStats()
	assert.Greater(t, cacheStats.TotalEntries, 0)
	
	queueStats := analyzer.GetQueueStats()
	assert.Greater(t, queueStats.TotalRequests, int64(0))
	
	// Test cleanup
	analyzer.Cleanup(1 * time.Hour)
	
	// Test shutdown
	err = analyzer.Shutdown(2 * time.Second)
	assert.NoError(t, err)
}

func testQualityAssessmentCoverage(t *testing.T) {
	// Test all analysis quality scenarios
	
	// Test complete quality
	completeQuality := &AnalysisQuality{
		IsComplete:           true,
		AnalysisMode:         AnalysisModeComplete,
		QualityScore:         1.0,
		MissingDependencies:  []string{},
		ImportErrors:         []ImportError{},
		EnhancementAvailable: false,
	}
	
	assert.True(t, completeQuality.IsComplete)
	assert.Equal(t, AnalysisModeComplete, completeQuality.AnalysisMode)
	
	// Test partial quality
	partialQuality := &AnalysisQuality{
		IsComplete:   false,
		AnalysisMode: AnalysisModePartial,
		QualityScore: 0.6,
		MissingDependencies: []string{
			"github.com/missing/dep1",
			"github.com/missing/dep2",
		},
		ImportErrors: []ImportError{
			{
				ImportPath: "github.com/missing/dep1",
				Error:      "could not import",
				Position:   "main.go:5:2",
				Severity:   "error",
			},
		},
		EnhancementAvailable: true,
	}
	
	assert.False(t, partialQuality.IsComplete)
	assert.Equal(t, AnalysisModePartial, partialQuality.AnalysisMode)
	assert.True(t, partialQuality.EnhancementAvailable)
	
	// Test failed quality
	failedQuality := &AnalysisQuality{
		IsComplete:           false,
		AnalysisMode:         AnalysisModeFailed,
		QualityScore:         0.0,
		EnhancementAvailable: false,
	}
	
	assert.Equal(t, AnalysisModeFailed, failedQuality.AnalysisMode)
	assert.False(t, failedQuality.EnhancementAvailable)
	
	// Test import error extraction
	errorMsg := "could not import github.com/test/package (invalid package name: \"\")"
	importPath := extractImportPathFromError(errorMsg)
	assert.Equal(t, "github.com/test/package", importPath)
	
	// Test empty error
	emptyImportPath := extractImportPathFromError("some other error")
	assert.Equal(t, "", emptyImportPath)
}

func testErrorHandlingCoverage(t *testing.T) {
	// Test error handling in various components
	
	// Test dependency checker errors
	checker := &SimpleDependencyChecker{}
	available, err := checker.AreDependenciesAvailable("/nonexistent/path", []string{"dep1"})
	assert.NoError(t, err) // Should not error, but return empty
	assert.Len(t, available, 0)
	
	// Test cache with invalid paths
	cache := NewAnalysisCache(checker)
	shouldRecalc, deps, err := cache.ShouldRecalculate(
		CacheKey{Type: CacheKeyTypePackage, PackagePath: "nonexistent"},
		"/nonexistent/path",
	)
	assert.NoError(t, err) // Should handle gracefully
	assert.True(t, shouldRecalc) // No cache, should recalculate
	assert.Nil(t, deps)
	
	// Test queue with full queue
	smallConfig := DependencyQueueConfig{
		MaxConcurrentDownloads: 1,
		QueueSize:             1, // Very small queue
		DownloadTimeout:       1 * time.Second,
	}
	queue := NewDependencyQueue(smallConfig)
	
	// Fill the queue
	req1 := DependencyDownloadRequest{
		WorkDir:      "/tmp",
		Dependencies: []string{"dep1"},
		CacheKey:     CacheKey{Type: CacheKeyTypePackage, PackagePath: "test1"},
		RequestID:    "req1",
		ResultChan:   make(chan DependencyDownloadResult, 1),
	}
	
	req2 := DependencyDownloadRequest{
		WorkDir:      "/tmp", 
		Dependencies: []string{"dep2"},
		CacheKey:     CacheKey{Type: CacheKeyTypePackage, PackagePath: "test2"},
		RequestID:    "req2",
		ResultChan:   make(chan DependencyDownloadResult, 1),
	}
	
	err = queue.SubmitDownloadRequest(req1)
	assert.NoError(t, err)
	
	// Second request should fail due to full queue
	err = queue.SubmitDownloadRequest(req2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue is full")
	
	// Test shutdown
	err = queue.Shutdown(2 * time.Second)
	assert.NoError(t, err)
}

// TestCoverageReport runs go test with coverage and generates a report
func TestCoverageReport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coverage report in short mode")
	}
	
	// This test doesn't do anything by itself, but when run with:
	// go test -coverprofile=coverage.out -covermode=atomic ./internal/analyzer
	// it will generate a coverage report that can be viewed with:
	// go tool cover -html=coverage.out -o coverage.html
	
	t.Log("Run with: go test -coverprofile=coverage.out ./internal/analyzer")
	t.Log("View with: go tool cover -html=coverage.out -o coverage.html")
}