package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackagesAnalyzer_Basic(t *testing.T) {
	// Create a temporary directory with a simple Go package
	tempDir, err := os.MkdirTemp("", "packages-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a simple Go file
	goFile := filepath.Join(tempDir, "main.go")
	goContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}

var testVar = 42

const TestConst = "test"

type TestType struct {
	Field string
}
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Create go.mod file
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module test-module

go 1.21
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Test packages analyzer
	packagesAnalyzer := NewPackagesAnalyzer(tempDir, nil)
	require.NotNil(t, packagesAnalyzer)

	// Test package analysis
	packageInfo, err := packagesAnalyzer.AnalyzePackageWithPackages("")
	require.NoError(t, err)
	require.NotNil(t, packageInfo)

	// Verify basic package info
	assert.Equal(t, "main", packageInfo.Name)
	assert.Greater(t, len(packageInfo.Files), 0)
	
	// Check that we have a file
	found := false
	for _, file := range packageInfo.Files {
		if file.Path == "main.go" && file.IsGo {
			found = true
			break
		}
	}
	assert.True(t, found, "Should have found main.go file")

	// Test single file analysis
	fileInfo, err := packagesAnalyzer.AnalyzeSingleFileWithPackages("main.go")
	require.NoError(t, err)
	require.NotNil(t, fileInfo)

	// Verify file info
	assert.Contains(t, fileInfo.Source, "Hello, world!")
	assert.Greater(t, len(fileInfo.References), 0, "Should have found references")
	
	// Should find some symbols (at least main function)
	assert.Greater(t, len(fileInfo.Symbols), 0, "Should have found symbols")
}

func TestPackagesAnalyzer_WithStandardAnalyzer_Compatibility(t *testing.T) {
	// Create a temporary directory with a simple Go package
	tempDir, err := os.MkdirTemp("", "compat-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a simple Go file
	goFile := filepath.Join(tempDir, "main.go")
	goContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Create go.mod file
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module test-module

go 1.21
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Test both analyzers and verify they produce similar results
	
	// Standard analyzer
	standardAnalyzer := New()
	standardPackageInfo, err := standardAnalyzer.AnalyzePackage(tempDir, "")
	require.NoError(t, err)

	// Packages analyzer
	packagesAnalyzer := New()
	packagesAnalyzer.WithPackagesSupport(tempDir, nil)
	packagesPackageInfo, err := packagesAnalyzer.AnalyzePackage(tempDir, "")
	require.NoError(t, err)

	// Both should identify the same package name
	assert.Equal(t, standardPackageInfo.Name, packagesPackageInfo.Name)
	
	// Both should find files
	assert.Greater(t, len(standardPackageInfo.Files), 0)
	assert.Greater(t, len(packagesPackageInfo.Files), 0)
	
	// Both should find symbols
	assert.Greater(t, len(standardPackageInfo.Symbols), 0)
	assert.Greater(t, len(packagesPackageInfo.Symbols), 0)
	
	t.Logf("Standard analyzer found %d symbols", len(standardPackageInfo.Symbols))
	t.Logf("Packages analyzer found %d symbols", len(packagesPackageInfo.Symbols))
}

func TestPackagesAnalyzer_WithIsolation(t *testing.T) {
	// This test verifies that packages analyzer works with isolated environments
	// Similar to repo manager isolation tests
	
	// Create a temporary directory with a Go package that imports external dependencies
	tempDir, err := os.MkdirTemp("", "packages-isolation-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a Go file that uses standard library imports
	goFile := filepath.Join(tempDir, "main.go")
	goContent := `package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	fmt.Println("Testing isolation")
	cwd, _ := os.Getwd()
	abs, _ := filepath.Abs(".")
	fmt.Printf("Working directory: %s, Absolute: %s\n", cwd, abs)
}
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Create go.mod file
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module isolation-test

go 1.21
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Create custom environment (simulating isolation)
	customEnv := os.Environ()
	customEnv = append(customEnv, "TEST_ISOLATION=true")

	// Test packages analyzer with custom environment
	packagesAnalyzer := NewPackagesAnalyzer(tempDir, customEnv)
	require.NotNil(t, packagesAnalyzer)

	// Analyze the package
	packageInfo, err := packagesAnalyzer.AnalyzePackageWithPackages("")
	require.NoError(t, err)
	require.NotNil(t, packageInfo)

	// Should successfully analyze the package even with custom environment
	assert.Equal(t, "main", packageInfo.Name)
	assert.Greater(t, len(packageInfo.Files), 0)
	assert.Greater(t, len(packageInfo.Symbols), 0)
	
	t.Logf("Successfully analyzed package with custom environment - found %d symbols", len(packageInfo.Symbols))
}

func TestPackagesAnalyzer_ErrorHandling(t *testing.T) {
	// Test error handling for non-existent directory
	packagesAnalyzer := NewPackagesAnalyzer("/nonexistent/path", nil)
	
	_, err := packagesAnalyzer.AnalyzePackageWithPackages("")
	assert.Error(t, err, "Should fail for non-existent path")
	
	// Test error handling for invalid package path
	tempDir, err := os.MkdirTemp("", "error-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	
	packagesAnalyzer = NewPackagesAnalyzer(tempDir, nil)
	
	_, err = packagesAnalyzer.AnalyzePackageWithPackages("nonexistent/package")
	assert.Error(t, err, "Should fail for non-existent package")
}