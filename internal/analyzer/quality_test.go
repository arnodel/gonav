package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalysisQuality_WithMissingDependencies(t *testing.T) {
	// Create a temporary directory with a Go package that has missing dependencies
	tempDir, err := os.MkdirTemp("", "quality-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create go.mod with external dependency
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module quality-test

go 1.21

require (
	github.com/nonexistent/fake-package v1.0.0
)
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Create Go file with missing import
	goFile := filepath.Join(tempDir, "main.go")
	goContent := `package main

import (
	"fmt"
	"github.com/nonexistent/fake-package/fakelib"
)

func main() {
	fake := fakelib.New()
	fmt.Println("Hello", fake)
}
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Test packages analyzer with quality assessment
	packagesAnalyzer := NewPackagesAnalyzer(tempDir, nil)
	require.NotNil(t, packagesAnalyzer)

	// Analyze with quality assessment
	response, err := packagesAnalyzer.AnalyzePackageWithQuality("")
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify quality assessment
	quality := response.Quality
	assert.NotNil(t, quality, "Quality assessment should be present")
	assert.False(t, quality.IsComplete, "Analysis should be marked as incomplete")
	assert.Equal(t, AnalysisModePartial, quality.AnalysisMode, "Should be partial analysis mode")
	assert.Greater(t, len(quality.MissingDependencies), 0, "Should have missing dependencies")
	assert.True(t, quality.EnhancementAvailable, "Enhancement should be available")
	assert.Greater(t, len(quality.ImportErrors), 0, "Should have import errors")
	assert.Less(t, quality.QualityScore, 1.0, "Quality score should be less than 1.0")

	// Verify we still got package info despite missing dependencies
	assert.NotNil(t, response.PackageInfo, "Should still have package info")
	assert.Equal(t, "main", response.PackageInfo.Name, "Package name should be correct")
	
	// Verify enhancement token is generated
	assert.NotEmpty(t, response.EnhancementToken, "Enhancement token should be generated")

	t.Logf("Analysis quality: mode=%s, score=%.2f, missing_deps=%v", 
		quality.AnalysisMode, quality.QualityScore, quality.MissingDependencies)
}

func TestAnalysisQuality_CompleteAnalysis(t *testing.T) {
	// Create a temporary directory with a simple Go package (no external dependencies)
	tempDir, err := os.MkdirTemp("", "quality-complete-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create go.mod
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module quality-complete-test

go 1.21
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Create Go file with only standard library imports
	goFile := filepath.Join(tempDir, "main.go")
	goContent := `package main

import (
	"fmt"
	"os"
)

func main() {
	cwd, _ := os.Getwd()
	fmt.Printf("Working directory: %s\n", cwd)
}
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Test packages analyzer with quality assessment
	packagesAnalyzer := NewPackagesAnalyzer(tempDir, nil)
	require.NotNil(t, packagesAnalyzer)

	// Analyze with quality assessment
	response, err := packagesAnalyzer.AnalyzePackageWithQuality("")
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify quality assessment for complete analysis
	quality := response.Quality
	assert.NotNil(t, quality, "Quality assessment should be present")
	assert.True(t, quality.IsComplete, "Analysis should be complete")
	assert.Equal(t, AnalysisModeComplete, quality.AnalysisMode, "Should be complete analysis mode")
	assert.Len(t, quality.MissingDependencies, 0, "Should have no missing dependencies")
	assert.False(t, quality.EnhancementAvailable, "Enhancement should not be available")
	assert.Len(t, quality.ImportErrors, 0, "Should have no import errors")
	assert.Equal(t, 1.0, quality.QualityScore, "Quality score should be 1.0")

	// Verify package info is present
	assert.NotNil(t, response.PackageInfo, "Should have package info")
	assert.Equal(t, "main", response.PackageInfo.Name, "Package name should be correct")
	
	// Enhancement token should be empty for complete analysis
	assert.Empty(t, response.EnhancementToken, "Enhancement token should be empty")

	t.Logf("Complete analysis quality: mode=%s, score=%.2f", 
		quality.AnalysisMode, quality.QualityScore)
}

func TestAnalysisQuality_FileAnalysis(t *testing.T) {
	// Create a temporary directory with a Go file that has mixed dependencies
	tempDir, err := os.MkdirTemp("", "quality-file-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create go.mod
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module quality-file-test

go 1.21

require (
	github.com/missing/dep v1.0.0
)
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Create Go file with both stdlib and missing imports
	goFile := filepath.Join(tempDir, "example.go")
	goContent := `package main

import (
	"fmt"
	"os"
	"github.com/missing/dep/somepackage"
)

func example() {
	cwd, _ := os.Getwd()
	missing := somepackage.New()
	fmt.Printf("Working directory: %s, missing: %v\n", cwd, missing)
}
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Test file analysis with quality assessment
	packagesAnalyzer := NewPackagesAnalyzer(tempDir, nil)
	require.NotNil(t, packagesAnalyzer)

	// Analyze single file with quality assessment
	response, err := packagesAnalyzer.AnalyzeSingleFileWithQuality("example.go")
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify quality assessment for file analysis
	quality := response.Quality
	assert.NotNil(t, quality, "Quality assessment should be present")
	assert.False(t, quality.IsComplete, "Analysis should be incomplete due to missing dependency")
	assert.Equal(t, AnalysisModePartial, quality.AnalysisMode, "Should be partial analysis mode")
	assert.Greater(t, len(quality.MissingDependencies), 0, "Should have missing dependencies")
	assert.True(t, quality.EnhancementAvailable, "Enhancement should be available")

	// Verify file info is present
	assert.NotNil(t, response.FileInfo, "Should have file info")
	assert.Greater(t, len(response.FileInfo.Source), 0, "Should have source content")
	
	t.Logf("File analysis quality: mode=%s, score=%.2f, missing_deps=%v", 
		quality.AnalysisMode, quality.QualityScore, quality.MissingDependencies)
}