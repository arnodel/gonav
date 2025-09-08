package analyzer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPackagesAnalyzer_extractRelativeFilePathFromCache(t *testing.T) {
	// Create a packages analyzer for testing the method
	pa := NewPackagesAnalyzer("/tmp", nil)

	tests := []struct {
		name     string
		input    string
		expected string
		desc     string
	}{
		{
			name:     "GoModCache_RootFile",
			input:    "/var/folders/1t/xd5sr7457bj8g748y4d4s78m0000gn/T/gonav-cache/isolated-env/gomodcache/github.com/arnodel/edit@v0.0.0-20220202110212-dfc8d7a13890/buffer.go",
			expected: "buffer.go",
			desc:     "Extract root-level file from gomodcache path",
		},
		{
			name:     "GoModCache_SubdirectoryFile",
			input:    "/var/folders/1t/xd5sr7457bj8g748y4d4s78m0000gn/T/gonav-cache/isolated-env/gomodcache/github.com/arnodel/edit@v0.0.0-20220202110212-dfc8d7a13890/subdir/helper.go",
			expected: "subdir/helper.go",
			desc:     "Extract subdirectory file from gomodcache path",
		},
		{
			name:     "GoModCache_NestedSubdirectory",
			input:    "/var/folders/1t/xd5sr7457bj8g748y4d4s78m0000gn/T/gonav-cache/isolated-env/gomodcache/github.com/gin-gonic/gin@v1.9.1/internal/json/jsoniter.go",
			expected: "internal/json/jsoniter.go",
			desc:     "Extract nested subdirectory file from gomodcache path",
		},
		{
			name:     "CustomCache_RootFile",
			input:    "/tmp/gonav-cache/github.com_arnodel_edit_v0.0.0-20220202110212-dfc8d7a13890/buffer.go",
			expected: "buffer.go",
			desc:     "Extract root-level file from custom gonav-cache path",
		},
		{
			name:     "CustomCache_SubdirectoryFile",
			input:    "/tmp/gonav-cache/github.com_arnodel_edit_v0.0.0-20220202110212-dfc8d7a13890/internal/helper.go",
			expected: "internal/helper.go",
			desc:     "Extract subdirectory file from custom gonav-cache path",
		},
		{
			name:     "StandardLibrary",
			input:    "/usr/local/go/src/fmt/print.go",
			expected: "",
			desc:     "Standard library path should return empty string",
		},
		{
			name:     "NoVersionInGoModCache",
			input:    "/var/folders/gomodcache/github.com/somemodule/file.go",
			expected: "",
			desc:     "GoModCache path without @ version should return empty string",
		},
		{
			name:     "NonCachePath",
			input:    "/some/random/path/to/file.go",
			expected: "",
			desc:     "Non-cache path should return empty string",
		},
		{
			name:     "EmptyPath",
			input:    "",
			expected: ".",
			desc:     "Empty path should return dot (filepath.Base behavior)",
		},
		{
			name:     "WindowsStylePath",
			input:    "C:\\tmp\\gonav-cache\\isolated-env\\gomodcache\\github.com\\arnodel\\edit@v0.0.0-20220202110212-dfc8d7a13890\\buffer.go",
			expected: "", // Non-Unix paths return empty string 
			desc:     "Windows-style path returns empty string (not handled by current logic)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pa.extractRelativeFilePathFromCache(tt.input)
			assert.Equal(t, tt.expected, result, tt.desc)
		})
	}
}

func TestPackagesAnalyzer_extractRelativeFilePathFromCache_EdgeCases(t *testing.T) {
	pa := NewPackagesAnalyzer("/tmp", nil)

	t.Run("MultipleAtSigns", func(t *testing.T) {
		// Test with multiple @ signs in path (should handle gracefully)
		input := "/tmp/gomodcache/github.com/user@domain/repo@v1.0.0@extra/file.go"
		result := pa.extractRelativeFilePathFromCache(input)
		// Current logic finds first @ in the path part and extracts from there
		assert.Equal(t, "repo@v1.0.0@extra/file.go", result)
	})

	t.Run("GoModCacheAtEnd", func(t *testing.T) {
		// Test path that ends with gomodcache
		input := "/var/folders/some/path/gomodcache"
		result := pa.extractRelativeFilePathFromCache(input)
		assert.Equal(t, "", result) // Returns empty string for non-matching paths
	})

	t.Run("NoFileAfterVersion", func(t *testing.T) {
		// Test path that ends after version (no file)
		input := "/tmp/gomodcache/github.com/user/repo@v1.0.0"
		result := pa.extractRelativeFilePathFromCache(input)
		assert.Equal(t, "", result) // No file after version, returns empty string
	})

	t.Run("MalformedGoModCachePath", func(t *testing.T) {
		// Test malformed gomodcache path
		input := "/tmp/gomodcache/invalid-module-name/file.go"
		result := pa.extractRelativeFilePathFromCache(input)
		assert.Equal(t, "", result) // Returns empty string since no @ version
	})
}

// TestPackagesAnalyzer_extractRelativeFilePathFromCache_Integration tests the fix
// in the context of the actual problem we solved - external method references
func TestPackagesAnalyzer_extractRelativeFilePathFromCache_Integration(t *testing.T) {
	pa := NewPackagesAnalyzer("/tmp", nil)

	// These are the actual path patterns we encountered when fixing the AppendLine issue
	realWorldCases := []struct {
		name        string
		cachePath   string
		expected    string
		description string
	}{
		{
			name:        "AppendLineMethodFix",
			cachePath:   "/var/folders/1t/xd5sr7457bj8g748y4d4s78m0000gn/T/gonav-cache/isolated-env/gomodcache/github.com/arnodel/edit@v0.0.0-20220202110212-dfc8d7a13890/buffer.go",
			expected:    "buffer.go",
			description: "The exact path pattern that was causing AppendLine navigation to fail",
		},
		{
			name:        "ExternalMethodInSubpackage", 
			cachePath:   "/var/folders/1t/xd5sr7457bj8g748y4d4s78m0000gn/T/gonav-cache/isolated-env/gomodcache/github.com/gin-gonic/gin@v1.9.1/internal/render/render.go",
			expected:    "internal/render/render.go",
			description: "External method in a subpackage that should preserve directory structure",
		},
		{
			name:        "TCellStyleFix",
			cachePath:   "/var/folders/1t/xd5sr7457bj8g748y4d4s78m0000gn/T/gonav-cache/isolated-env/gomodcache/github.com/gdamore/tcell/v2@v2.4.0/style.go",
			expected:    "style.go",
			description: "Another real external reference that was in the logs",
		},
	}

	for _, tc := range realWorldCases {
		t.Run(tc.name, func(t *testing.T) {
			result := pa.extractRelativeFilePathFromCache(tc.cachePath)
			assert.Equal(t, tc.expected, result, tc.description)

			// Verify the result is a valid relative path (no absolute path markers)
			assert.False(t, strings.HasPrefix(result, "/"), "Result should not be an absolute path")
			assert.False(t, strings.Contains(result, "gomodcache"), "Result should not contain cache directory names")
			assert.False(t, strings.Contains(result, "isolated-env"), "Result should not contain isolation directory names")
			assert.False(t, strings.Contains(result, "@"), "Result should not contain version markers")
		})
	}
}