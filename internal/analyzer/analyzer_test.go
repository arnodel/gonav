package analyzer

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"testing"
)

func TestScopeExtraction(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		expectedScopes []ScopeInfo
	}{
		{
			name: "simple function",
			source: `package main

func main() {
	x := 1
}`,
			expectedScopes: []ScopeInfo{
				{
					ID:   "/main",
					Type: "function",
					Name: "main",
					Range: Range{
						Start: Position{Line: 3, Column: 6},
						End:   Position{Line: 5, Column: 1},
					},
				},
			},
		},
		{
			name: "function with if block",
			source: `package main

func test() {
	x := 1
	if x > 0 {
		y := 2
	}
}`,
			expectedScopes: []ScopeInfo{
				{
					ID:   "/test",
					Type: "function",
					Name: "test",
					Range: Range{
						Start: Position{Line: 3, Column: 6},
						End:   Position{Line: 8, Column: 1},
					},
				},
				{
					ID:   "/test/if_1",
					Type: "block",
					Range: Range{
						Start: Position{Line: 5, Column: 12},
						End:   Position{Line: 7, Column: 2},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			analyzer := New()
			scopes, err := analyzer.extractScopes(file, fset, nil)
			if err != nil {
				t.Fatalf("extractScopes failed: %v", err)
			}

			if len(scopes) != len(tt.expectedScopes) {
				t.Fatalf("Expected %d scopes, got %d", len(tt.expectedScopes), len(scopes))
			}

			for i, expected := range tt.expectedScopes {
				actual := scopes[i]
				if actual.ID != expected.ID {
					t.Errorf("Scope %d: expected ID %q, got %q", i, expected.ID, actual.ID)
				}
				if actual.Type != expected.Type {
					t.Errorf("Scope %d: expected Type %q, got %q", i, expected.Type, actual.Type)
				}
				if actual.Name != expected.Name {
					t.Errorf("Scope %d: expected Name %q, got %q", i, expected.Name, actual.Name)
				}
			}
		})
	}
}

func TestDefinitionExtraction(t *testing.T) {
	tests := []struct {
		name                string
		source              string
		expectedDefinitions []Definition
	}{
		{
			name: "global and local variables",
			source: `package main

var globalVar = 1

func main() {
	localVar := 2
}`,
			expectedDefinitions: []Definition{
				{
					ID:        "def_1",
					Name:      "globalVar",
					Type:      "var",
					Line:      3,
					Column:    5,
					ScopeID:   "/",
					Signature: "int",
				},
				{
					ID:        "def_2",
					Name:      "main",
					Type:      "func",
					Line:      5,
					Column:    6,
					ScopeID:   "/",
					Signature: "func",
				},
				{
					ID:        "def_3",
					Name:      "localVar",
					Type:      "var",
					Line:      6,
					Column:    2,
					ScopeID:   "/main",
					Signature: "int",
				},
			},
		},
		{
			name: "function definitions in global scope",
			source: `package main

func quoteLuaVal(x int) string {
	return "test"
}

func anotherFunc() {
	localVar := 1
}`,
			expectedDefinitions: []Definition{
				{
					ID:        "def_1",
					Name:      "quoteLuaVal",
					Type:      "func",
					Line:      3,
					Column:    6,
					ScopeID:   "/",
					Signature: "func",
				},
				{
					ID:        "def_2", 
					Name:      "x",
					Type:      "var", 
					Line:      3,
					Column:    18,
					ScopeID:   "/quoteLuaVal",
					Signature: "int",
				},
				{
					ID:        "def_3", 
					Name:      "anotherFunc",
					Type:      "func",
					Line:      7,
					Column:    6,
					ScopeID:   "/",
					Signature: "func",
				},
				{
					ID:        "def_4",
					Name:      "localVar",
					Type:      "var", 
					Line:      8,
					Column:    2,
					ScopeID:   "/anotherFunc",
					Signature: "int",
				},
			},
		},
		{
			name: "imported type references",
			source: `package main

import (
	"bytes"
	"net/http"
)

type MyStruct struct {
	buf    bytes.Buffer
	client *http.Client
}`,
			expectedDefinitions: []Definition{
				{
					ID:        "def_1",
					Name:      "MyStruct",
					Type:      "type",
					Line:      8,
					Column:    6,
					ScopeID:   "/",
					Signature: "type",
				},
				{
					ID:        "def_2",
					Name:      "buf",
					Type:      "var",
					Line:      9,
					Column:    2,
					ScopeID:   "/",
					Signature: "int",
				},
				{
					ID:        "def_3",
					Name:      "client",
					Type:      "var",
					Line:      10,
					Column:    2,
					ScopeID:   "/",
					Signature: "int",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			analyzer := New()
			
			// Set up type checking to get proper type information
			config := &types.Config{
				Importer: importer.Default(),
				Error: func(err error) {
					// Ignore errors for test
				},
			}
			
			info := &types.Info{
				Defs: make(map[*ast.Ident]types.Object),
				Uses: make(map[*ast.Ident]types.Object),
			}
			
			_, _ = config.Check("main", fset, []*ast.File{file}, info)
			// Continue even if type checking fails
			
			definitions, err := analyzer.extractDefinitions(file, fset, info)
			if err != nil {
				t.Fatalf("extractDefinitions failed: %v", err)
			}

			if len(definitions) != len(tt.expectedDefinitions) {
				t.Fatalf("Expected %d definitions, got %d", len(tt.expectedDefinitions), len(definitions))
			}

			for i, expected := range tt.expectedDefinitions {
				actual := definitions[i]
				if actual.Name != expected.Name {
					t.Errorf("Definition %d: expected Name %q, got %q", i, expected.Name, actual.Name)
				}
				if actual.Type != expected.Type {
					t.Errorf("Definition %d: expected Type %q, got %q", i, expected.Type, actual.Type)
				}
				if actual.ScopeID != expected.ScopeID {
					t.Errorf("Definition %d: expected ScopeID %q, got %q", i, expected.ScopeID, actual.ScopeID)
				}
			}
		})
	}
}

func TestReferenceExtraction(t *testing.T) {
	tests := []struct {
		name               string
		source             string
		expectedReferences int // Just count for now
	}{
		{
			name: "imported type references",
			source: `package main

import (
	"bytes"
	"net/http"
)

type MyStruct struct {
	buf    bytes.Buffer
	client *http.Client
}

func test() {
	var b bytes.Buffer
	var c *http.Client
}`,
			expectedReferences: 4, // bytes.Buffer (x2), http.Client (x2)
		},
		{
			name: "pointer type references with StarExpr",
			source: `package main

import (
	"context"
	"net/http"
)

type MyStruct struct {
	ctx    context.Context
	client *http.Client
	server *http.Server
}

func NewStruct() *MyStruct {
	return &MyStruct{
		ctx: context.Background(),
	}
}`,
			expectedReferences: 6, // context.Context, http.Client, http.Server, MyStruct, MyStruct, context.Background
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := New()
			
			// Write the source to a temp file and analyze it
			tmpDir := t.TempDir()
			testFile := tmpDir + "/test.go"
			err := os.WriteFile(testFile, []byte(tt.source), 0644)
			if err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			// Analyze the file (this will extract references)
			fileInfo, err := analyzer.AnalyzeSingleFile(tmpDir, "test.go")
			if err != nil {
				t.Fatalf("AnalyzeSingleFile failed: %v", err)
			}

			referenceCount := len(fileInfo.References)
			t.Logf("Found %d references", referenceCount)
			
			// Log all references for debugging
			for i, ref := range fileInfo.References {
				targetInfo := "no target"
				if ref.Target != nil {
					targetInfo = fmt.Sprintf("target: %s (pkg: %s, isStdLib: %t)", 
						ref.Target.Name, ref.Target.Package, ref.Target.IsStdLib)
				}
				t.Logf("Reference %d: %s at %d:%d - %s", i, ref.Name, ref.Line, ref.Column, targetInfo)
			}

			if referenceCount < tt.expectedReferences {
				t.Errorf("Expected at least %d references, got %d", tt.expectedReferences, referenceCount)
			}
		})
	}
}

func TestSymbolCollectionDefinitionsOnly(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		expectedSymbols []string // Only symbol names that should be in the symbols map
	}{
		{
			name: "definitions vs references",
			source: `package main

import "fmt"

type MyType struct {
	field int
}

func myFunc() {
	var local MyType  // Reference to MyType, not a definition
	fmt.Println(local)
}`,
			expectedSymbols: []string{"MyType", "field", "myFunc", "local"}, // Only definitions
		},
		{
			name: "external references not in symbols",
			source: `package main

import "fmt"

func main() {
	fmt.Println("test")  // fmt.Println is a reference, not a definition
}`,
			expectedSymbols: []string{"main"}, // Only local definitions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := New()
			
			// Write the source to a temp file and analyze it
			tmpDir := t.TempDir()
			testFile := tmpDir + "/test.go"
			err := os.WriteFile(testFile, []byte(tt.source), 0644)
			if err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			// Analyze the file
			fileInfo, err := analyzer.AnalyzeSingleFile(tmpDir, "test.go")
			if err != nil {
				t.Fatalf("AnalyzeSingleFile failed: %v", err)
			}

			// Check that symbols map only contains expected definitions
			if len(fileInfo.Symbols) != len(tt.expectedSymbols) {
				t.Errorf("Expected %d symbols, got %d", len(tt.expectedSymbols), len(fileInfo.Symbols))
				
				// Log what we got for debugging
				t.Logf("Expected symbols: %v", tt.expectedSymbols)
				actualSymbols := make([]string, 0, len(fileInfo.Symbols))
				for name := range fileInfo.Symbols {
					actualSymbols = append(actualSymbols, name)
				}
				t.Logf("Actual symbols: %v", actualSymbols)
			}

			// Verify each expected symbol exists and is a definition (not "external")
			for _, expectedName := range tt.expectedSymbols {
				symbol, exists := fileInfo.Symbols[expectedName]
				if !exists {
					t.Errorf("Expected symbol %q not found in symbols map", expectedName)
					continue
				}
				
				// Verify it's a proper definition type (not "external")
				validTypes := []string{"function", "type", "var", "const", "method", "field", "func"}
				isValidType := false
				for _, validType := range validTypes {
					if symbol.Type == validType {
						isValidType = true
						break
					}
				}
				
				if !isValidType {
					t.Errorf("Symbol %q has invalid type %q, should be a definition type", expectedName, symbol.Type)
				}
			}
		})
	}
}

func TestStandardLibraryClassification(t *testing.T) {
	tests := []struct {
		name           string
		importPath     string
		moduleInfo     *ModuleInfo
		expectedStdLib bool
	}{
		{
			name:           "actual standard library package",
			importPath:     "fmt",
			moduleInfo:     &ModuleInfo{ModulePath: "github.com/example/project"},
			expectedStdLib: true,
		},
		{
			name:           "another standard library package",
			importPath:     "os",
			moduleInfo:     &ModuleInfo{ModulePath: "github.com/example/project"},
			expectedStdLib: true,
		},
		{
			name:           "external package with domain",
			importPath:     "github.com/other/package",
			moduleInfo:     &ModuleInfo{ModulePath: "github.com/example/project"},
			expectedStdLib: false,
		},
		{
			name:           "internal package within current module",
			importPath:     "runtime",
			moduleInfo:     &ModuleInfo{ModulePath: "github.com/arnodel/golua"},
			expectedStdLib: false, // This was the bug we fixed
		},
		{
			name:           "another internal package",
			importPath:     "lib",
			moduleInfo:     &ModuleInfo{ModulePath: "github.com/arnodel/golua"},
			expectedStdLib: false,
		},
		{
			name:           "main package",
			importPath:     "main",
			moduleInfo:     &ModuleInfo{ModulePath: "github.com/example/project"},
			expectedStdLib: false,
		},
		{
			name:           "empty import path",
			importPath:     "",
			moduleInfo:     &ModuleInfo{ModulePath: "github.com/example/project"},
			expectedStdLib: false,
		},
	}

	analyzer := New()
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.IsStandardLibraryImportWithContext(tt.importPath, tt.moduleInfo)
			if result != tt.expectedStdLib {
				t.Errorf("IsStandardLibraryImportWithContext(%q, %v) = %t, expected %t", 
					tt.importPath, tt.moduleInfo.ModulePath, result, tt.expectedStdLib)
			}
		})
	}
}