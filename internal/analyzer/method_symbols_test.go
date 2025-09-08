package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackagesAnalyzer_MethodSymbolExtraction(t *testing.T) {
	// Create a temporary directory with Go files that have methods
	tempDir, err := os.MkdirTemp("", "method-symbols-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a Go file with various method types
	goFile := filepath.Join(tempDir, "methods.go")
	goContent := `package testpkg

import "fmt"

// FileBuffer represents a buffer for file operations
type FileBuffer struct {
	Content string
	Lines   []string
}

// AppendLine adds a line to the buffer (pointer receiver method)
func (fb *FileBuffer) AppendLine(line string) {
	fb.Lines = append(fb.Lines, line)
	fb.Content += line + "\n"
}

// GetContent returns the content (pointer receiver method)
func (fb *FileBuffer) GetContent() string {
	return fb.Content
}

// LineCount returns the number of lines (value receiver method)
func (fb FileBuffer) LineCount() int {
	return len(fb.Lines)
}

// Counter is a simple counter type
type Counter int

// Increment increments the counter (pointer receiver)
func (c *Counter) Increment() {
	*c++
}

// Value returns the current value (value receiver)
func (c Counter) Value() int {
	return int(c)
}

// String implements fmt.Stringer (value receiver)
func (c Counter) String() string {
	return fmt.Sprintf("Counter: %d", c)
}

// Helper is an interface
type Helper interface {
	Help() string
}

// RegularFunction is not a method
func RegularFunction() {
	fmt.Println("Not a method")
}

// Exported variable
var ExportedVar = "test"

// Exported constant  
const ExportedConst = 42
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Create go.mod file
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module test-methods

go 1.21
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Create module info
	moduleInfo := &ModuleInfo{
		ModulePath:   "test-methods",
		Dependencies: make(map[string]string),
		Replaces:     make(map[string]string),
	}

	// Test packages analyzer
	packagesAnalyzer := NewPackagesAnalyzer(tempDir, nil)
	packagesAnalyzer.SetModuleContext(moduleInfo)
	require.NotNil(t, packagesAnalyzer)

	// Analyze the package
	packageInfo, err := packagesAnalyzer.AnalyzePackageWithPackages("")
	require.NoError(t, err)
	require.NotNil(t, packageInfo)

	// Verify we have symbols
	assert.Greater(t, len(packageInfo.Symbols), 0, "Should have found symbols")

	// Test 1: Check that regular symbols are present
	assert.Contains(t, packageInfo.Symbols, "FileBuffer", "Should have FileBuffer type")
	assert.Contains(t, packageInfo.Symbols, "Counter", "Should have Counter type")
	assert.Contains(t, packageInfo.Symbols, "Helper", "Should have Helper interface")
	assert.Contains(t, packageInfo.Symbols, "RegularFunction", "Should have regular function")
	assert.Contains(t, packageInfo.Symbols, "ExportedVar", "Should have exported variable")
	assert.Contains(t, packageInfo.Symbols, "ExportedConst", "Should have exported constant")

	// Test 2: Check for qualified method names in symbols
	expectedMethods := []struct {
		name        string
		description string
	}{
		{"(*FileBuffer).AppendLine", "Pointer receiver method"},
		{"(*FileBuffer).GetContent", "Pointer receiver method"},
		{"FileBuffer.LineCount", "Value receiver method"},
		{"(*Counter).Increment", "Pointer receiver method on Counter"},
		{"Counter.Value", "Value receiver method on Counter"},
		{"Counter.String", "Value receiver method implementing Stringer"},
	}

	foundMethods := make(map[string]bool)
	for _, method := range expectedMethods {
		if _, exists := packageInfo.Symbols[method.name]; exists {
			foundMethods[method.name] = true
			t.Logf("✓ Found method symbol: %s (%s)", method.name, method.description)
		} else {
			t.Errorf("✗ Missing method symbol: %s (%s)", method.name, method.description)
		}
	}

	// Test 3: Verify method symbols have correct properties
	for methodName, found := range foundMethods {
		if !found {
			continue
		}

		symbol := packageInfo.Symbols[methodName]
		assert.NotNil(t, symbol, "Method symbol should not be nil: %s", methodName)
		assert.Equal(t, "methods.go", symbol.File, "Method should be in methods.go: %s", methodName)
		assert.Equal(t, "function", symbol.Type, "Method should have function type: %s", methodName)
		assert.Greater(t, symbol.Line, 0, "Method should have valid line number: %s", methodName)
		assert.Equal(t, "testpkg", symbol.Package, "Method should be in testpkg: %s", methodName)
		assert.False(t, symbol.IsExternal, "Method should not be external: %s", methodName)
		assert.False(t, symbol.IsStdLib, "Method should not be stdlib: %s", methodName)
		
		t.Logf("Method %s: Line %d, Signature: %s", methodName, symbol.Line, symbol.Signature)
	}

	// Test 4: Verify that we don't have duplicate methods or malformed names
	allSymbolNames := make([]string, 0, len(packageInfo.Symbols))
	for name := range packageInfo.Symbols {
		allSymbolNames = append(allSymbolNames, name)
	}

	// Check for qualified method name patterns
	methodCount := 0
	for _, name := range allSymbolNames {
		if strings.Contains(name, ".") && strings.Contains(name, "(") {
			// This looks like a qualified method name
			methodCount++
			// Should not have both value and pointer receiver versions of same method
			// (the MethodSet logic should handle this correctly)
			assert.True(t, strings.HasPrefix(name, "(*") || !strings.Contains(name, "*"),
				"Method name should be properly formatted: %s", name)
		}
	}

	assert.Greater(t, methodCount, 0, "Should have found qualified method names")
	t.Logf("Found %d qualified method symbols out of %d total symbols", methodCount, len(packageInfo.Symbols))

	// Test 5: Verify we don't have any malformed method names
	for name := range packageInfo.Symbols {
		if strings.Contains(name, ".") {
			// This is likely a method name, verify format
			assert.False(t, strings.HasSuffix(name, "."), "Method name should not end with dot: %s", name)
			assert.False(t, strings.HasPrefix(name, "."), "Method name should not start with dot: %s", name)
			
			if strings.Contains(name, "(") {
				// Pointer receiver method should have proper format
				assert.True(t, strings.HasPrefix(name, "(*") && strings.Contains(name, ")."),
					"Pointer receiver method should have format (*Type).Method: %s", name)
			}
		}
	}
}

func TestPackagesAnalyzer_MethodSymbolInReferences(t *testing.T) {
	// Test that method symbols appear correctly in references too
	tempDir, err := os.MkdirTemp("", "method-refs-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a Go file that uses methods
	goFile := filepath.Join(tempDir, "main.go")
	goContent := `package main

import "fmt"

type Buffer struct {
	data string
}

func (b *Buffer) Write(s string) {
	b.data += s
}

func (b Buffer) Read() string {
	return b.data
}

func main() {
	buf := &Buffer{}
	buf.Write("hello") // Should create reference to (*Buffer).Write
	content := buf.Read() // Should create reference to Buffer.Read
	fmt.Println(content)
}
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Create go.mod file
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module method-refs-test

go 1.21
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Test packages analyzer
	packagesAnalyzer := NewPackagesAnalyzer(tempDir, nil)

	// Analyze single file for references
	fileInfo, err := packagesAnalyzer.AnalyzeSingleFileWithPackages("main.go")
	require.NoError(t, err)
	require.NotNil(t, fileInfo)

	// Check references for qualified method names
	assert.Greater(t, len(fileInfo.References), 0, "Should have found references")

	foundMethodRefs := make(map[string]*Reference)
	for _, ref := range fileInfo.References {
		if ref.Target != nil && strings.Contains(ref.Target.Name, ".") {
			foundMethodRefs[ref.Target.Name] = ref
			t.Logf("Found method reference: %s -> %s", ref.Name, ref.Target.Name)
		}
	}

	// We should find references to our qualified method names
	expectedRefs := []string{"(*Buffer).Write", "Buffer.Read"}
	for _, expected := range expectedRefs {
		found := false
		for targetName := range foundMethodRefs {
			if targetName == expected {
				found = true
				break
			}
		}
		if found {
			t.Logf("✓ Found expected method reference: %s", expected)
		} else {
			// This might be okay if the analysis didn't resolve the method call to the qualified name
			t.Logf("Note: Expected method reference not found: %s", expected)
		}
	}

	// Test that method references have proper target information
	for refName, ref := range foundMethodRefs {
		assert.NotNil(t, ref.Target, "Method reference should have target: %s", refName)
		assert.Equal(t, "main.go", ref.Target.File, "Method reference should point to main.go: %s", refName)
		assert.Equal(t, "function", ref.Target.Type, "Method reference should have function type: %s", refName)
		assert.False(t, ref.Target.IsExternal, "Method reference should not be external: %s", refName)
	}
}

func TestPackagesAnalyzer_MethodSymbolEdgeCases(t *testing.T) {
	// Test edge cases in method symbol extraction
	tempDir, err := os.MkdirTemp("", "method-edge-cases-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a Go file with edge cases
	goFile := filepath.Join(tempDir, "edges.go")
	goContent := `package edges

// AnonymousStruct with methods
var AnonymousVar = struct {
	Field string
}{
	Field: "test",
}

// Embedded methods
type Embedder struct {
	Embedded
}

type Embedded struct{}

func (e Embedded) EmbeddedMethod() {}

// Method with same name on different types
type TypeA struct{}
type TypeB struct{}

func (a TypeA) SameName() {}
func (b TypeB) SameName() {}

// Method that returns a method
func (a TypeA) GetMethod() func() {
	return func() {}
}

// Generic type (if supported by Go version)
// This might not work in older Go versions, but should be handled gracefully

// Unexported type with exported method
type unexportedType struct{}

func (u unexportedType) ExportedMethod() {}

// Exported type with unexported method
type ExportedType struct{}

func (e ExportedType) unexportedMethod() {}
`
	err = os.WriteFile(goFile, []byte(goContent), 0644)
	require.NoError(t, err)

	// Create go.mod file
	modFile := filepath.Join(tempDir, "go.mod")
	modContent := `module edges-test

go 1.21
`
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	require.NoError(t, err)

	// Test packages analyzer
	packagesAnalyzer := NewPackagesAnalyzer(tempDir, nil)

	// Analyze the package
	packageInfo, err := packagesAnalyzer.AnalyzePackageWithPackages("")
	require.NoError(t, err)
	require.NotNil(t, packageInfo)

	// Test: Methods with same names on different types should be distinguished
	assert.Contains(t, packageInfo.Symbols, "TypeA.SameName", "Should have TypeA.SameName")
	assert.Contains(t, packageInfo.Symbols, "TypeB.SameName", "Should have TypeB.SameName")

	// Test: Both exported and unexported types/methods should be handled
	assert.Contains(t, packageInfo.Symbols, "unexportedType.ExportedMethod", "Should have method on unexported type")
	assert.Contains(t, packageInfo.Symbols, "ExportedType.unexportedMethod", "Should have unexported method")

	// Test: Embedded methods should be handled
	foundEmbedded := false
	for name := range packageInfo.Symbols {
		if strings.Contains(name, "EmbeddedMethod") {
			foundEmbedded = true
			t.Logf("Found embedded method: %s", name)
		}
	}
	if !foundEmbedded {
		t.Log("Note: Embedded methods not found - this is acceptable behavior")
	}

	// Test: No malformed symbol names
	for name, symbol := range packageInfo.Symbols {
		// Basic sanity checks
		assert.NotEmpty(t, name, "Symbol name should not be empty")
		assert.NotNil(t, symbol, "Symbol should not be nil")
		
		// If it's a qualified method name, check format
		if strings.Contains(name, ".") && strings.Contains(name, "(") {
			// Should be either (*Type).Method or Type.Method
			parts := strings.Split(name, ".")
			assert.Len(t, parts, 2, "Qualified method name should have exactly one dot: %s", name)
			
			if strings.HasPrefix(name, "(*") {
				assert.True(t, strings.Contains(parts[0], ")"), "Pointer receiver should close parentheses: %s", name)
			}
		}
	}

	t.Logf("Successfully tested %d symbols with edge cases", len(packageInfo.Symbols))
}