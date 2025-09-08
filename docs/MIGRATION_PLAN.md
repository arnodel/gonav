# Go Packages Integration Migration Plan

## Overview

This document outlines the migration plan to integrate `golang.org/x/tools/go/packages` into our Go navigation server while maintaining backward compatibility and adding Go environment isolation.

## Current State Analysis

### Existing Architecture

Our current system uses a **hybrid caching approach** that is actually quite sophisticated:

1. **Primary download method**: `go mod download` (downloads to Go's official module cache)
2. **Symlink system**: Creates symlinks from our cache (`/tmp/gonav-cache/`) to Go module cache
3. **Fallback method**: Git clone for modules not available via Go proxy
4. **Manual analysis**: Custom AST parsing and type checking setup

**Example current cache structure:**
```
/tmp/gonav-cache/
├── github.com_arnodel_golua_v0.1.0 -> /Users/arno/go/pkg/mod/github.com/arnodel/golua@v0.1.0
└── github.com_arnodel_edit_v0.0.0-20220202110212-dfc8d7a13890 -> /Users/arno/go/pkg/mod/github.com/arnodel/edit@v0.0.0-20220202110212-dfc8d7a13890
```

### Key Findings

1. **Package Fetching**: `go/packages` does NOT handle fetching - it only works with locally available modules
2. **Version Handling**: Direct `@version` syntax doesn't work, must use `Config.Dir` approach
3. **Local Analysis Still Needed**: For function-local variables, closures, non-exported symbols
4. **Environment Isolation**: Fully possible with `GOMODCACHE`, `GOCACHE`, `GOPATH` environment variables

## Problems with Current Implementation

1. **Complex Manual Setup**: Significant code for manual `go/parser` + `go/types` configuration
2. **Incomplete Type Information**: Missing cross-package type resolution and imports
3. **Host Environment Pollution**: Downloads affect user's Go module cache
4. **Limited Standard Library Detection**: Recently improved but could be more robust
5. **Error-Prone**: Manual type checking setup is fragile

## Benefits of Migration

### Using `golang.org/x/tools/go/packages`

**Package-Level Analysis Benefits:**
- ✅ Robust module resolution and dependency tracking
- ✅ Complete type information with proper import resolution  
- ✅ Automatic handling of build constraints and Go modules
- ✅ Better cross-package reference resolution
- ✅ Simplified code (replace ~200 lines with ~20 lines)
- ✅ Future-proof (official Go tooling, actively maintained)

**Environment Isolation Benefits:**
- ✅ No interference with host Go environment
- ✅ Clean, reproducible builds
- ✅ Easy cleanup and management
- ✅ Support for multiple Go versions

**Preserved Manual Analysis:**
- ✅ Function-local variables and parameters
- ✅ Closure-scoped variables  
- ✅ Block-scoped variables (if, for, switch blocks)
- ✅ Non-exported package members

## Migration Strategy

### Phase 1: Add Go Environment Isolation (No Breaking Changes)

**Goal**: Isolate all Go operations without changing external APIs

**Steps:**

1. **Create Isolated Go Environment System**
   ```go
   type IsolatedGoEnv struct {
       baseDir    string
       gomodcache string
       gocache    string  
       gopath     string
       env        []string
   }
   ```

2. **Modify Repository Manager**
   - Update `downloadWithGoMod()` to use isolated environment
   - Keep existing symlink creation for backward compatibility
   - Add cleanup methods for isolated environments

3. **Testing**
   - Ensure all existing functionality works
   - Verify isolation (no pollution of host environment)
   - Test with multiple concurrent downloads

**Files to Modify:**
- `internal/repo/manager.go`: Add isolation environment setup
- Add new `internal/env/isolation.go`: Environment management

### Phase 2: Integrate go/packages for Package Analysis

**Goal**: Replace manual `go/parser` + `go/types` setup with `go/packages` while maintaining API compatibility

**Steps:**

1. **Add go/packages Dependency**
   ```bash
   go get golang.org/x/tools/go/packages
   ```

2. **Create Hybrid Analyzer**
   ```go
   type HybridAnalyzer struct {
       // Existing fields
       fset           *token.FileSet
       packages       map[string]*PackageInfo
       stdLibCache    map[string]bool
       
       // New fields
       isolatedEnv    *IsolatedGoEnv
   }
   ```

3. **Implement Package-Level Analysis with go/packages**
   - Create `analyzePackageWithGoPackages()` method
   - Use `Config.Dir` pointing to isolated module cache
   - Extract exported symbols, imports, dependencies
   - Convert results to existing `PackageInfo` structure

4. **Maintain File-Level Manual Analysis**
   - Keep existing `AnalyzeSingleFile()` for local symbols
   - Use manual AST traversal for function-local analysis
   - Combine results from both approaches

**Files to Modify:**
- `internal/analyzer/analyzer.go`: Add hybrid analysis methods
- `go.mod`: Add go/packages dependency

### Phase 3: Version-Specific Package Loading

**Goal**: Enable analysis of specific module versions using go/packages

**Steps:**

1. **Implement Version Resolution**
   ```go
   func (a *HybridAnalyzer) AnalyzeVersionedPackage(moduleAtVersion, packagePath string) (*PackageInfo, error) {
       // Parse module@version
       modulePath, version := parseModuleAtVersion(moduleAtVersion)
       
       // Ensure module exists in isolated cache
       cacheDir, err := a.isolatedEnv.EnsureModule(modulePath, version)
       if err != nil {
           return nil, err
       }
       
       // Use go/packages with Config.Dir
       cfg := &packages.Config{
           Mode: packages.LoadAllSyntax,
           Dir:  cacheDir,
           Env:  a.isolatedEnv.Environment(),
       }
       
       pattern := "./" + packagePath
       if packagePath == "" {
           pattern = "."
       }
       
       pkgs, err := packages.Load(cfg, pattern)
       // Process results...
   }
   ```

2. **Update API Endpoints**
   - Modify package analysis endpoints to use new hybrid approach
   - Ensure response format remains compatible

3. **Performance Optimization**
   - Cache package analysis results
   - Reuse isolated environments when possible

**Files to Modify:**
- `internal/analyzer/analyzer.go`: Add versioned analysis
- `cmd/server/main.go`: Update API endpoints

### Phase 4: Enhanced Standard Library Detection

**Goal**: Leverage go/packages for more robust standard library detection

**Steps:**

1. **Enhanced Detection Logic**
   ```go
   func (a *HybridAnalyzer) IsStandardLibraryWithGoPackages(importPath string) bool {
       // Use go/packages to load and check if pkg.Module is nil (stdlib)
       cfg := &packages.Config{
           Mode: packages.NeedModule,
           Env:  a.isolatedEnv.Environment(),
       }
       pkgs, err := packages.Load(cfg, importPath)
       if err != nil || len(pkgs) == 0 {
           return false
       }
       return pkgs[0].Module == nil // Standard library packages have no Module
   }
   ```

2. **Fallback Strategy**
   - Keep existing `go/build` approach as fallback
   - Use cached results to avoid repeated calls

**Files to Modify:**
- `internal/analyzer/analyzer.go`: Enhance standard library detection

### Phase 5: Testing and Optimization

**Goal**: Ensure stability and optimize performance

**Steps:**

1. **Comprehensive Testing**
   - Test with various module versions
   - Test cross-package references
   - Test standard library detection
   - Test environment isolation

2. **Performance Benchmarking**
   - Compare analysis speed vs current implementation
   - Optimize caching strategies
   - Profile memory usage

3. **Documentation Updates**
   - Update API documentation
   - Add migration notes
   - Update README with new capabilities

**Files to Modify:**
- `internal/analyzer/analyzer_test.go`: Add comprehensive tests
- `docs/API.md`: Update documentation

## Detailed Implementation Plan

### Environment Isolation Implementation

```go
// internal/env/isolation.go
package env

import (
    "os"
    "os/exec"
    "path/filepath"
)

type IsolatedGoEnv struct {
    baseDir    string
    gomodcache string
    gocache    string
    gopath     string
    env        []string
}

func NewIsolatedGoEnv(baseDir string) (*IsolatedGoEnv, error) {
    env := &IsolatedGoEnv{
        baseDir:    baseDir,
        gomodcache: filepath.Join(baseDir, "gomodcache"),
        gocache:    filepath.Join(baseDir, "gocache"),
        gopath:     filepath.Join(baseDir, "gopath"),
    }
    
    // Create directories
    os.MkdirAll(env.gomodcache, 0755)
    os.MkdirAll(env.gocache, 0755)
    os.MkdirAll(env.gopath, 0755)
    
    // Setup environment variables
    env.env = append(os.Environ(),
        fmt.Sprintf("GOMODCACHE=%s", env.gomodcache),
        fmt.Sprintf("GOCACHE=%s", env.gocache),
        fmt.Sprintf("GOPATH=%s", env.gopath),
        "GO111MODULE=on",
    )
    
    return env, nil
}

func (e *IsolatedGoEnv) DownloadModule(moduleAtVersion string) (string, error) {
    cmd := exec.Command("go", "mod", "download", "-json", moduleAtVersion)
    cmd.Env = e.env
    
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    
    var downloadInfo GoModDownloadInfo
    if err := json.Unmarshal(output, &downloadInfo); err != nil {
        return "", err
    }
    
    return downloadInfo.Dir, nil
}

func (e *IsolatedGoEnv) Environment() []string {
    return e.env
}

func (e *IsolatedGoEnv) ModuleCachePath() string {
    return e.gomodcache
}

func (e *IsolatedGoEnv) Cleanup() error {
    return os.RemoveAll(e.baseDir)
}
```

### Hybrid Analyzer Implementation

```go
// internal/analyzer/hybrid.go
package analyzer

import (
    "golang.org/x/tools/go/packages"
    "gonav/internal/env"
)

func (a *PackageAnalyzer) analyzeWithGoPackages(modulePath, version, packagePath string) (*PackageInfo, error) {
    // Get isolated module cache path
    moduleAtVersion := modulePath + "@" + version
    cacheDir := filepath.Join(a.isolatedEnv.ModuleCachePath(), moduleAtVersion)
    
    cfg := &packages.Config{
        Mode: packages.LoadAllSyntax,
        Dir:  cacheDir,
        Env:  a.isolatedEnv.Environment(),
    }
    
    // Load package
    pattern := "./" + packagePath
    if packagePath == "" {
        pattern = "."
    }
    
    pkgs, err := packages.Load(cfg, pattern)
    if err != nil {
        return nil, err
    }
    
    if len(pkgs) == 0 {
        return nil, fmt.Errorf("no packages found")
    }
    
    pkg := pkgs[0]
    
    // Convert to our PackageInfo format
    packageInfo := &PackageInfo{
        Name:    pkg.Name,
        Path:    packagePath,
        Files:   convertGoFilesToFileEntries(pkg.GoFiles),
        Symbols: make(map[string]*Symbol),
    }
    
    // Extract symbols from type information
    if pkg.TypesInfo != nil {
        a.extractSymbolsFromTypesInfo(pkg.TypesInfo, pkg.Fset, packageInfo)
    }
    
    // Extract symbols from AST (for additional coverage)
    for _, file := range pkg.Syntax {
        a.extractSymbolsFromAST(file, pkg.Fset, packageInfo)
    }
    
    return packageInfo, nil
}

func (a *PackageAnalyzer) extractSymbolsFromTypesInfo(info *types.Info, fset *token.FileSet, pkgInfo *PackageInfo) {
    // Extract definitions
    for ident, obj := range info.Defs {
        if obj == nil {
            continue
        }
        
        pos := fset.Position(ident.Pos())
        
        symbol := &Symbol{
            Name:    obj.Name(),
            Type:    getObjectType(obj),
            File:    getRelativePath(pos.Filename, pkgInfo.Path),
            Line:    pos.Line,
            Column:  pos.Column,
            Package: obj.Pkg().Name(),
        }
        
        // Add type signature
        if signature := getTypeSignature(obj); signature != "" {
            symbol.Signature = signature
        }
        
        // Only include exported symbols in main symbols map
        if obj.Exported() {
            pkgInfo.Symbols[obj.Name()] = symbol
        }
    }
}
```

## Risk Assessment

### Low Risk
- **Environment Isolation**: Well-tested approach, easy to implement
- **Backward Compatibility**: Maintaining existing APIs and symlink structure

### Medium Risk  
- **go/packages Integration**: New dependency, but well-documented and stable
- **Performance Impact**: Need to benchmark, but should be faster overall

### High Risk
- **Breaking Changes**: Must ensure existing clients continue to work
- **Complex Hybrid Logic**: Combining manual analysis with go/packages needs careful design

## Success Metrics

1. **Functionality**: All existing API endpoints work unchanged
2. **Performance**: Analysis speed same or better than current implementation  
3. **Isolation**: No pollution of host Go environment during testing
4. **Accuracy**: Better symbol resolution and cross-references
5. **Maintainability**: Reduced code complexity, especially in analyzer

## Timeline

- **Phase 1** (Environment Isolation): 2-3 days
- **Phase 2** (go/packages Integration): 3-4 days  
- **Phase 3** (Version-Specific Loading): 2-3 days
- **Phase 4** (Enhanced Standard Library): 1-2 days
- **Phase 5** (Testing & Optimization): 2-3 days

**Total Estimated Time**: 10-15 days

## Rollback Plan

If issues arise during migration:

1. **Phase 1**: Revert environment isolation, fallback to current system
2. **Phase 2+**: Keep environment isolation, disable go/packages analysis
3. **Feature Flags**: Implement toggles to switch between old/new analysis methods

## Dependencies

- `golang.org/x/tools/go/packages`: Main integration dependency
- Go 1.21+: Required for latest packages functionality
- Current dependencies: No breaking changes required

## Conclusion

This migration plan provides a structured approach to integrate `go/packages` while maintaining stability and backward compatibility. The hybrid approach allows us to get the best of both worlds: robust package-level analysis from go/packages and detailed file-level analysis from our manual approach.

The phased implementation reduces risk and allows for incremental testing and validation at each step.