# Go Packages Integration - Detailed Implementation Plan

## Implementation Philosophy

Each stage must result in a **fully working system** that can be:
- ✅ Built and run successfully
- ✅ Tested with existing test suite
- ✅ Deployed to production if needed
- ✅ Rolled back safely if issues arise

## Stage 1: Environment Isolation Foundation
**Duration**: 2-3 days  
**Goal**: Add isolation without changing any existing behavior

### Implementation Steps

#### 1.1 Create Environment Isolation Module (Day 1)

**New File**: `internal/env/isolation.go`
```go
package env

import (
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
)

type IsolatedEnv struct {
    BaseDir    string
    GoModCache string
    GoCache    string
    GoPath     string
    env        []string
}

func NewIsolated(baseDir string) (*IsolatedEnv, error) {
    env := &IsolatedEnv{
        BaseDir:    baseDir,
        GoModCache: filepath.Join(baseDir, "gomodcache"),
        GoCache:    filepath.Join(baseDir, "gocache"),
        GoPath:     filepath.Join(baseDir, "gopath"),
    }
    
    // Create directories
    if err := os.MkdirAll(env.GoModCache, 0755); err != nil {
        return nil, err
    }
    if err := os.MkdirAll(env.GoCache, 0755); err != nil {
        return nil, err
    }
    if err := os.MkdirAll(env.GoPath, 0755); err != nil {
        return nil, err
    }
    
    // Setup environment
    env.env = append(os.Environ(),
        fmt.Sprintf("GOMODCACHE=%s", env.GoModCache),
        fmt.Sprintf("GOCACHE=%s", env.GoCache),
        fmt.Sprintf("GOPATH=%s", env.GoPath),
        "GO111MODULE=on",
    )
    
    return env, nil
}

func (e *IsolatedEnv) Environment() []string {
    return e.env
}

func (e *IsolatedEnv) ExecCommand(name string, args ...string) *exec.Cmd {
    cmd := exec.Command(name, args...)
    cmd.Env = e.env
    return cmd
}

func (e *IsolatedEnv) Cleanup() error {
    return os.RemoveAll(e.BaseDir)
}
```

**Tests**: `internal/env/isolation_test.go`
```go
func TestIsolatedEnv(t *testing.T) {
    tempDir, err := os.MkdirTemp("", "gonav-test-*")
    require.NoError(t, err)
    defer os.RemoveAll(tempDir)
    
    env, err := NewIsolated(tempDir)
    require.NoError(t, err)
    
    // Test environment variables are set
    envVars := env.Environment()
    assert.Contains(t, envVars, fmt.Sprintf("GOMODCACHE=%s", env.GoModCache))
    
    // Test directory creation
    assert.DirExists(t, env.GoModCache)
    assert.DirExists(t, env.GoCache)
    
    // Test command execution
    cmd := env.ExecCommand("go", "env", "GOMODCACHE")
    output, err := cmd.Output()
    require.NoError(t, err)
    assert.Contains(t, string(output), env.GoModCache)
}
```

#### 1.2 Add Isolation Option to Repository Manager (Day 1-2)

**Modify**: `internal/repo/manager.go`

Add optional isolation support:
```go
type Manager struct {
    cacheDir    string
    repos       map[string]string
    isolatedEnv *env.IsolatedEnv // NEW: Optional isolation
}

type ManagerOption func(*Manager) // NEW: Options pattern

func WithIsolation(isolated bool) ManagerOption { // NEW
    return func(m *Manager) {
        if isolated {
            envDir := filepath.Join(m.cacheDir, "isolated-env")
            isolatedEnv, err := env.NewIsolated(envDir)
            if err != nil {
                fmt.Printf("Warning: failed to create isolated environment: %v\n", err)
                return
            }
            m.isolatedEnv = isolatedEnv
        }
    }
}

func NewManager(opts ...ManagerOption) *Manager { // MODIFIED
    cacheDir := filepath.Join(os.TempDir(), "gonav-cache")
    os.MkdirAll(cacheDir, 0755)

    m := &Manager{
        cacheDir: cacheDir,
        repos:    make(map[string]string),
    }
    
    // Apply options
    for _, opt := range opts {
        opt(m)
    }
    
    return m
}

func (m *Manager) downloadWithGoMod(modulePath, version string) (string, error) {
    moduleAtVersion := modulePath + "@" + version
    
    var cmd *exec.Cmd
    if m.isolatedEnv != nil {
        // NEW: Use isolated environment
        cmd = m.isolatedEnv.ExecCommand("go", "mod", "download", "-json", moduleAtVersion)
    } else {
        // EXISTING: Use host environment
        cmd = exec.Command("go", "mod", "download", "-json", moduleAtVersion)
    }
    
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("go mod download failed: %w", err)
    }

    var downloadInfo GoModDownloadInfo
    if err := json.Unmarshal(output, &downloadInfo); err != nil {
        return "", fmt.Errorf("failed to parse go mod download output: %w", err)
    }

    if downloadInfo.Dir == "" {
        return "", fmt.Errorf("go mod download did not provide directory path")
    }

    if _, err := os.Stat(downloadInfo.Dir); os.IsNotExist(err) {
        return "", fmt.Errorf("go mod download directory does not exist: %s", downloadInfo.Dir)
    }

    return downloadInfo.Dir, nil
}
```

#### 1.3 Update Server to Support Isolation Flag (Day 2)

**Modify**: `cmd/server/main.go`

Add command line flag:
```go
var (
    port     = flag.Int("port", 8080, "Port to run server on")
    isolated = flag.Bool("isolated", false, "Use isolated Go environment") // NEW
)

func main() {
    flag.Parse()
    
    var managerOpts []repo.ManagerOption
    if *isolated {
        managerOpts = append(managerOpts, repo.WithIsolation(true))
        fmt.Println("Running with isolated Go environment")
    }
    
    repoManager := repo.NewManager(managerOpts...) // MODIFIED
    
    // Rest remains the same...
}
```

#### 1.4 Testing Strategy (Day 2-3)

**Test Script**: `test_isolation.sh`
```bash
#!/bin/bash
set -e

echo "=== Stage 1: Testing Environment Isolation ==="

# Test 1: Normal mode (existing behavior)
echo "1. Testing normal mode..."
go build -o bin/gonav-test cmd/server/main.go
./bin/gonav-test -port=8081 &
SERVER_PID=$!
sleep 2

# Test existing API
curl -s "http://localhost:8081/api/repo/github.com%2Farnodel%2Fgolua%40v0.1.0" | jq '.moduleAtVersion'
curl -s "http://localhost:8081/api/package/github.com%2Farnodel%2Fgolua%40v0.1.0/runtime" | jq '.name'

kill $SERVER_PID

# Test 2: Isolated mode (new behavior)
echo "2. Testing isolated mode..."
./bin/gonav-test -port=8082 -isolated=true &
SERVER_PID=$!
sleep 2

# Same API calls should work
curl -s "http://localhost:8082/api/repo/github.com%2Farnodel%2Fgolua%40v0.1.0" | jq '.moduleAtVersion'
curl -s "http://localhost:8082/api/package/github.com%2Farnodel%2Fgolua%40v0.1.0/runtime" | jq '.name'

kill $SERVER_PID

# Test 3: Verify isolation (no pollution)
echo "3. Verifying host environment not polluted..."
# Check that isolated downloads don't affect host GOMODCACHE
go env GOMODCACHE
ls "$(go env GOMODCACHE)" | grep -v "github.com/arnodel/golua@v0.1.0" || echo "Isolation working!"

echo "✅ Stage 1 Complete: Environment isolation working"
```

**Unit Tests**: Update existing tests to verify both modes work
```go
func TestManagerIsolation(t *testing.T) {
    // Test normal mode
    normalManager := repo.NewManager()
    repoInfo, err := normalManager.LoadRepository("github.com/arnodel/golua@v0.1.0")
    require.NoError(t, err)
    assert.Equal(t, "github.com/arnodel/golua@v0.1.0", repoInfo.ModuleAtVersion)
    
    // Test isolated mode  
    isolatedManager := repo.NewManager(repo.WithIsolation(true))
    repoInfo2, err := isolatedManager.LoadRepository("github.com/arnodel/golua@v0.1.0")
    require.NoError(t, err)
    assert.Equal(t, "github.com/arnodel/golua@v0.1.0", repoInfo2.ModuleAtVersion)
    
    // Both should produce same API response
    assert.Equal(t, repoInfo.ModuleAtVersion, repoInfo2.ModuleAtVersion)
}
```

### Stage 1 Success Criteria
- ✅ All existing tests pass
- ✅ Server runs in both normal and isolated modes  
- ✅ API responses identical in both modes
- ✅ Isolation flag prevents host environment pollution
- ✅ No performance degradation

---

## Stage 2: go/packages Integration (Hybrid Approach)
**Duration**: 3-4 days  
**Goal**: Add go/packages analysis alongside existing system

### Implementation Steps

#### 2.1 Add go/packages Dependency (Day 1)

```bash
go get golang.org/x/tools/go/packages
go mod tidy
```

**Verify**: Ensure project still builds and tests pass

#### 2.2 Create Hybrid Analyzer (Day 1-2)

**New File**: `internal/analyzer/hybrid.go`
```go
package analyzer

import (
    "golang.org/x/tools/go/packages"
    "gonav/internal/env"
)

// HybridAnalyzer extends PackageAnalyzer with go/packages support
type HybridAnalyzer struct {
    *PackageAnalyzer // Embed existing analyzer
    isolatedEnv      *env.IsolatedEnv
    useGoPackages    bool // Feature flag
}

func NewHybrid(isolatedEnv *env.IsolatedEnv, useGoPackages bool) *HybridAnalyzer {
    return &HybridAnalyzer{
        PackageAnalyzer: New(), // Use existing analyzer
        isolatedEnv:     isolatedEnv,
        useGoPackages:   useGoPackages,
    }
}

// AnalyzePackageHybrid tries go/packages first, falls back to existing method
func (h *HybridAnalyzer) AnalyzePackageHybrid(repoPath, packagePath string) (*PackageInfo, error) {
    if h.useGoPackages && h.isolatedEnv != nil {
        if pkgInfo, err := h.analyzeWithGoPackages(repoPath, packagePath); err == nil {
            return pkgInfo, nil
        }
        // Log error but continue with fallback
        fmt.Printf("go/packages analysis failed, falling back to existing method: %v\n", err)
    }
    
    // Fallback to existing method
    return h.PackageAnalyzer.AnalyzePackage(repoPath, packagePath)
}

func (h *HybridAnalyzer) analyzeWithGoPackages(repoPath, packagePath string) (*PackageInfo, error) {
    // Determine module cache path from repoPath
    cacheDir := repoPath
    if h.isolatedEnv != nil {
        // If using isolated env, repoPath might be a symlink to isolated cache
        if resolvedPath, err := filepath.EvalSymlinks(repoPath); err == nil {
            cacheDir = resolvedPath
        }
    }
    
    cfg := &packages.Config{
        Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports | 
              packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
        Dir: cacheDir,
    }
    
    if h.isolatedEnv != nil {
        cfg.Env = h.isolatedEnv.Environment()
    }
    
    // Load package
    pattern := "./" + packagePath
    if packagePath == "" {
        pattern = "."
    }
    
    pkgs, err := packages.Load(cfg, pattern)
    if err != nil {
        return nil, fmt.Errorf("packages.Load failed: %w", err)
    }
    
    if len(pkgs) == 0 {
        return nil, fmt.Errorf("no packages found for pattern %s", pattern)
    }
    
    pkg := pkgs[0]
    if len(pkg.Errors) > 0 {
        return nil, fmt.Errorf("package has errors: %v", pkg.Errors[0])
    }
    
    // Convert to our PackageInfo format
    return h.convertGoPackageToPackageInfo(pkg, packagePath)
}

func (h *HybridAnalyzer) convertGoPackageToPackageInfo(pkg *packages.Package, packagePath string) (*PackageInfo, error) {
    pkgInfo := &PackageInfo{
        Name:    pkg.Name,
        Path:    packagePath,
        Files:   h.convertGoFilesToFileEntries(pkg.GoFiles, pkg.CompiledGoFiles),
        Symbols: make(map[string]*Symbol),
    }
    
    // Extract symbols from types info (better than AST parsing)
    if pkg.TypesInfo != nil && pkg.Types != nil {
        h.extractSymbolsFromTypesInfo(pkg.TypesInfo, pkg.Fset, pkg.Types, pkgInfo)
    }
    
    return pkgInfo, nil
}

func (h *HybridAnalyzer) extractSymbolsFromTypesInfo(info *types.Info, fset *token.FileSet, typePkg *types.Package, pkgInfo *PackageInfo) {
    // Extract definitions - only exported symbols for main package info
    for ident, obj := range info.Defs {
        if obj == nil || !obj.Exported() {
            continue
        }
        
        pos := fset.Position(ident.Pos())
        relPath, err := filepath.Rel(pkgInfo.Path, pos.Filename)
        if err != nil {
            relPath = filepath.Base(pos.Filename)
        }
        
        symbol := &Symbol{
            Name:    obj.Name(),
            Type:    h.getObjectType(obj),
            File:    relPath,
            Line:    pos.Line,
            Column:  pos.Column,
            Package: typePkg.Name(),
        }
        
        // Add signature information
        if sig := h.getTypeSignature(obj); sig != "" {
            symbol.Signature = sig
        }
        
        pkgInfo.Symbols[obj.Name()] = symbol
    }
}

func (h *HybridAnalyzer) getObjectType(obj types.Object) string {
    switch obj.(type) {
    case *types.Func:
        return "function"
    case *types.TypeName:
        return "type"
    case *types.Var:
        return "var"
    case *types.Const:
        return "const"
    default:
        return "unknown"
    }
}

func (h *HybridAnalyzer) getTypeSignature(obj types.Object) string {
    return obj.Type().String()
}

func (h *HybridAnalyzer) convertGoFilesToFileEntries(goFiles, compiledFiles []string) []FileEntry {
    fileSet := make(map[string]bool)
    var entries []FileEntry
    
    // Add all go files
    for _, file := range goFiles {
        if !fileSet[file] {
            entries = append(entries, FileEntry{
                Path: filepath.Base(file),
                IsGo: true,
            })
            fileSet[file] = true
        }
    }
    
    return entries
}
```

#### 2.3 Update Analyzer Factory (Day 2)

**Modify**: `internal/analyzer/analyzer.go`

Add factory method:
```go
// Factory method to create appropriate analyzer
func NewAnalyzerForManager(manager *repo.Manager) AnalyzerInterface {
    // Check if manager has isolated environment
    if isolatedEnv := manager.GetIsolatedEnv(); isolatedEnv != nil {
        // Create hybrid analyzer with go/packages enabled
        return NewHybrid(isolatedEnv, true)
    }
    
    // Fallback to existing analyzer
    return New()
}

// Interface to allow both analyzers
type AnalyzerInterface interface {
    AnalyzePackage(repoPath, packagePath string) (*PackageInfo, error)
    AnalyzeSingleFile(repoPath, filePath string) (*FileInfo, error)
    // ... other methods
}

// Ensure both analyzers implement the interface
var _ AnalyzerInterface = (*PackageAnalyzer)(nil)
var _ AnalyzerInterface = (*HybridAnalyzer)(nil)
```

**Add to Manager**: `internal/repo/manager.go`
```go
func (m *Manager) GetIsolatedEnv() *env.IsolatedEnv {
    return m.isolatedEnv
}
```

#### 2.4 Update Server Integration (Day 3)

**Modify**: `cmd/server/main.go`

Update to use analyzer factory:
```go
func main() {
    flag.Parse()
    
    var managerOpts []repo.ManagerOption
    if *isolated {
        managerOpts = append(managerOpts, repo.WithIsolation(true))
        fmt.Println("Running with isolated Go environment and go/packages integration")
    }
    
    repoManager := repo.NewManager(managerOpts...)
    analyzer := analyzer.NewAnalyzerForManager(repoManager) // NEW: Factory method
    
    // Rest of server setup uses analyzer interface...
}
```

#### 2.5 Feature Flag System (Day 3)

**Add environment variable**: `GO_PACKAGES_ENABLED=true/false`

```go
func shouldUseGoPackages() bool {
    return os.Getenv("GO_PACKAGES_ENABLED") == "true"
}
```

#### 2.6 Testing Strategy (Day 4)

**Test Script**: `test_hybrid.sh`
```bash
#!/bin/bash
set -e

echo "=== Stage 2: Testing Hybrid go/packages Integration ==="

# Test 1: Existing behavior (go/packages disabled)
echo "1. Testing with go/packages disabled..."
export GO_PACKAGES_ENABLED=false
go test ./internal/analyzer/... -v

# Test 2: go/packages enabled
echo "2. Testing with go/packages enabled..."  
export GO_PACKAGES_ENABLED=true
go test ./internal/analyzer/... -v

# Test 3: API compatibility
echo "3. Testing API response compatibility..."
./bin/gonav-test -port=8083 -isolated=true &
SERVER_PID=$!
sleep 2

# Test that responses have same structure
RESPONSE1=$(curl -s "http://localhost:8083/api/package/github.com%2Farnodel%2Fgolua%40v0.1.0/runtime")
echo $RESPONSE1 | jq '.symbols | keys | length' # Should have symbols

kill $SERVER_PID

echo "✅ Stage 2 Complete: Hybrid analysis working"
```

**Comparison Tests**: `internal/analyzer/hybrid_test.go`
```go
func TestHybridVsOriginal(t *testing.T) {
    tempDir, err := os.MkdirTemp("", "gonav-hybrid-test-*")
    require.NoError(t, err)
    defer os.RemoveAll(tempDir)
    
    isolatedEnv, err := env.NewIsolated(tempDir)
    require.NoError(t, err)
    
    // Download test module
    manager := repo.NewManager(repo.WithIsolation(true))
    repoInfo, err := manager.LoadRepository("github.com/arnodel/golua@v0.1.0")
    require.NoError(t, err)
    
    // Test both analyzers
    originalAnalyzer := New()
    hybridAnalyzer := NewHybrid(isolatedEnv, true)
    
    // Analyze same package
    originalResult, err1 := originalAnalyzer.AnalyzePackage(repoInfo.Files[0].Path, "runtime")
    hybridResult, err2 := hybridAnalyzer.AnalyzePackageHybrid(repoInfo.Files[0].Path, "runtime")
    
    require.NoError(t, err1)
    require.NoError(t, err2)
    
    // Compare results
    assert.Equal(t, originalResult.Name, hybridResult.Name)
    assert.Equal(t, originalResult.Path, hybridResult.Path)
    
    // Hybrid should have same or more symbols (better analysis)
    assert.GreaterOrEqual(t, len(hybridResult.Symbols), len(originalResult.Symbols))
    
    // Key symbols should be present in both
    for name := range originalResult.Symbols {
        if originalResult.Symbols[name].Type == "function" { // Focus on functions
            assert.Contains(t, hybridResult.Symbols, name, 
                "Hybrid analyzer missing function: %s", name)
        }
    }
}
```

### Stage 2 Success Criteria
- ✅ Both analyzers produce compatible results
- ✅ go/packages provides richer type information
- ✅ Fallback to original analyzer works seamlessly
- ✅ Feature flag allows switching between methods
- ✅ API responses maintain same JSON structure

---

## Stage 3: Version-Specific Package Loading
**Duration**: 2-3 days
**Goal**: Enable direct module@version analysis

### Implementation Steps

#### 3.1 Version-Specific Analysis Method (Day 1)

**Add to HybridAnalyzer**: `internal/analyzer/hybrid.go`
```go
// AnalyzeVersionedPackage analyzes a specific version directly
func (h *HybridAnalyzer) AnalyzeVersionedPackage(moduleAtVersion, packagePath string) (*PackageInfo, error) {
    modulePath, version, err := h.parseModuleAtVersion(moduleAtVersion)
    if err != nil {
        return nil, err
    }
    
    // Ensure module is available in isolated cache
    cacheDir, err := h.ensureModuleInCache(modulePath, version)
    if err != nil {
        return nil, err
    }
    
    if !h.useGoPackages {
        // Fallback to existing logic using cache directory
        return h.PackageAnalyzer.AnalyzePackage(cacheDir, packagePath)
    }
    
    // Use go/packages with module cache directory
    cfg := &packages.Config{
        Mode: packages.LoadAllSyntax,
        Dir:  cacheDir,
    }
    
    if h.isolatedEnv != nil {
        cfg.Env = h.isolatedEnv.Environment()
    }
    
    // Load specific package
    pattern := "./" + packagePath
    if packagePath == "" {
        pattern = "."
    }
    
    pkgs, err := packages.Load(cfg, pattern)
    if err != nil {
        return nil, fmt.Errorf("failed to load package %s: %w", pattern, err)
    }
    
    if len(pkgs) == 0 {
        return nil, fmt.Errorf("no packages found for %s in %s", pattern, moduleAtVersion)
    }
    
    return h.convertGoPackageToPackageInfo(pkgs[0], packagePath)
}

func (h *HybridAnalyzer) ensureModuleInCache(modulePath, version string) (string, error) {
    if h.isolatedEnv == nil {
        return "", fmt.Errorf("isolated environment required for version-specific analysis")
    }
    
    moduleAtVersion := modulePath + "@" + version
    expectedPath := filepath.Join(h.isolatedEnv.GoModCache, moduleAtVersion)
    
    // Check if already exists
    if stat, err := os.Stat(expectedPath); err == nil && stat.IsDir() {
        return expectedPath, nil
    }
    
    // Download to isolated cache
    cmd := h.isolatedEnv.ExecCommand("go", "mod", "download", "-json", moduleAtVersion)
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to download %s: %w", moduleAtVersion, err)
    }
    
    var downloadInfo repo.GoModDownloadInfo
    if err := json.Unmarshal(output, &downloadInfo); err != nil {
        return "", fmt.Errorf("failed to parse download info: %w", err)
    }
    
    return downloadInfo.Dir, nil
}

func (h *HybridAnalyzer) parseModuleAtVersion(moduleAtVersion string) (string, string, error) {
    parts := strings.Split(moduleAtVersion, "@")
    if len(parts) != 2 {
        return "", "", fmt.Errorf("invalid module@version format: %s", moduleAtVersion)
    }
    return parts[0], parts[1], nil
}
```

#### 3.2 Update API Endpoints (Day 1-2)

**Modify**: `cmd/server/main.go`
Add new endpoint for direct version analysis:
```go
func setupAPIRoutes(repoManager *repo.Manager, analyzer analyzer.AnalyzerInterface) {
    // Existing routes...
    
    // NEW: Direct version-specific package analysis
    http.HandleFunc("/api/package-direct/", func(w http.ResponseWriter, r *http.Request) {
        handlePackageDirectAnalysis(w, r, analyzer)
    })
}

func handlePackageDirectAnalysis(w http.ResponseWriter, r *http.Request, analyzer analyzer.AnalyzerInterface) {
    // Parse URL: /api/package-direct/{moduleAtVersion}/{packagePath}
    urlParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/package-direct/"), "/")
    if len(urlParts) < 1 {
        http.Error(w, "Invalid URL format", http.StatusBadRequest)
        return
    }
    
    moduleAtVersion, err := url.QueryUnescape(urlParts[0])
    if err != nil {
        http.Error(w, "Invalid module@version encoding", http.StatusBadRequest)
        return
    }
    
    var packagePath string
    if len(urlParts) > 1 {
        packagePath = strings.Join(urlParts[1:], "/")
    }
    
    // Check if analyzer supports version-specific analysis
    if hybridAnalyzer, ok := analyzer.(*analyzer.HybridAnalyzer); ok {
        packageInfo, err := hybridAnalyzer.AnalyzeVersionedPackage(moduleAtVersion, packagePath)
        if err != nil {
            http.Error(w, fmt.Sprintf("Analysis failed: %v", err), http.StatusInternalServerError)
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(packageInfo)
        return
    }
    
    // Fallback: not supported with current analyzer
    http.Error(w, "Version-specific analysis not supported", http.StatusNotImplemented)
}
```

#### 3.3 Testing Strategy (Day 2-3)

**Test Script**: `test_versioned.sh`
```bash
#!/bin/bash
set -e

echo "=== Stage 3: Testing Version-Specific Analysis ==="

export GO_PACKAGES_ENABLED=true
./bin/gonav-test -port=8084 -isolated=true &
SERVER_PID=$!
sleep 3

# Test 1: Direct version analysis
echo "1. Testing direct version analysis..."
RESPONSE=$(curl -s "http://localhost:8084/api/package-direct/github.com%2Farnodel%2Fgolua%40v0.1.0/runtime")
echo $RESPONSE | jq '.name'
echo $RESPONSE | jq '.symbols | keys | length'

# Test 2: Compare with traditional approach
echo "2. Comparing with traditional approach..."
TRADITIONAL=$(curl -s "http://localhost:8084/api/package/github.com%2Farnodel%2Fgolua%40v0.1.0/runtime")
DIRECT=$(curl -s "http://localhost:8084/api/package-direct/github.com%2Farnodel%2Fgolua%40v0.1.0/runtime")

# Both should have same package name
TRAD_NAME=$(echo $TRADITIONAL | jq -r '.name')
DIRECT_NAME=$(echo $DIRECT | jq -r '.name')

if [ "$TRAD_NAME" = "$DIRECT_NAME" ]; then
    echo "✓ Package names match: $TRAD_NAME"
else
    echo "✗ Package names differ: $TRAD_NAME vs $DIRECT_NAME"
    exit 1
fi

kill $SERVER_PID

echo "✅ Stage 3 Complete: Version-specific analysis working"
```

**Unit Tests**: `internal/analyzer/versioned_test.go`
```go
func TestVersionedPackageAnalysis(t *testing.T) {
    tempDir, err := os.MkdirTemp("", "gonav-versioned-test-*")
    require.NoError(t, err)
    defer os.RemoveAll(tempDir)
    
    isolatedEnv, err := env.NewIsolated(tempDir)
    require.NoError(t, err)
    
    analyzer := NewHybrid(isolatedEnv, true)
    
    // Test analyzing a specific version directly
    packageInfo, err := analyzer.AnalyzeVersionedPackage("github.com/arnodel/golua@v0.1.0", "runtime")
    require.NoError(t, err)
    
    assert.Equal(t, "runtime", packageInfo.Name)
    assert.Greater(t, len(packageInfo.Symbols), 0)
    
    // Verify key runtime symbols are present
    expectedSymbols := []string{"Runtime", "Thread", "Value"}
    for _, symbol := range expectedSymbols {
        assert.Contains(t, packageInfo.Symbols, symbol, "Missing expected symbol: %s", symbol)
    }
}
```

### Stage 3 Success Criteria
- ✅ Direct version analysis works without pre-loading repository
- ✅ New API endpoint provides same data structure
- ✅ Performance is acceptable (modules cached after first use)
- ✅ Both traditional and direct methods produce consistent results

---

## Stage 4: Enhanced Standard Library Detection
**Duration**: 1-2 days
**Goal**: Leverage go/packages for more robust stdlib detection

### Implementation Steps

#### 4.1 Enhanced Detection Methods (Day 1)

**Add to HybridAnalyzer**: `internal/analyzer/hybrid.go`
```go
// Enhanced standard library detection using go/packages
func (h *HybridAnalyzer) IsStandardLibraryEnhanced(importPath string, moduleInfo *ModuleInfo) bool {
    // Fast path: use existing cached detection first
    if cached, exists := h.stdLibCache[importPath]; exists {
        return cached
    }
    
    var result bool
    
    if h.useGoPackages && h.isolatedEnv != nil {
        result = h.detectStdLibWithGoPackages(importPath)
    } else {
        // Fallback to existing method
        result = h.PackageAnalyzer.IsStandardLibraryImportWithContext(importPath, moduleInfo)
    }
    
    h.stdLibCache[importPath] = result
    return result
}

func (h *HybridAnalyzer) detectStdLibWithGoPackages(importPath string) bool {
    cfg := &packages.Config{
        Mode: packages.NeedModule | packages.NeedName,
        Env:  h.isolatedEnv.Environment(),
    }
    
    pkgs, err := packages.Load(cfg, importPath)
    if err != nil || len(pkgs) == 0 {
        // Fallback to heuristic
        return !strings.Contains(importPath, ".")
    }
    
    pkg := pkgs[0]
    
    // Standard library packages have no Module field
    if pkg.Module == nil {
        return true
    }
    
    // Additional check: Go standard library modules have specific characteristics
    if pkg.Module.Path == "std" || pkg.Module.Path == "" {
        return true
    }
    
    return false
}
```

#### 4.2 Update Symbol Classification (Day 1)

```go
// Update reference extraction to use enhanced detection
func (h *HybridAnalyzer) classifyReference(importPath string, moduleInfo *ModuleInfo) (string, bool) {
    if h.IsStandardLibraryEnhanced(importPath, moduleInfo) {
        return "stdlib", true
    }
    
    if moduleInfo != nil && !moduleInfo.IsExternalImport(importPath) {
        return "internal", false
    }
    
    return "external", false
}
```

#### 4.3 Testing Strategy (Day 2)

**Test Script**: `test_stdlib_detection.sh`
```bash
#!/bin/bash
set -e

echo "=== Stage 4: Testing Enhanced Standard Library Detection ==="

export GO_PACKAGES_ENABLED=true
./bin/gonav-test -port=8085 -isolated=true &
SERVER_PID=$!
sleep 3

# Test package with mix of stdlib and external imports
RESPONSE=$(curl -s "http://localhost:8085/api/package-direct/github.com%2Farnodel%2Fgolua%40v0.1.0/runtime")

# Check that references are properly classified
echo $RESPONSE | jq '.references[] | select(.type == "stdlib") | .name' | head -5
echo $RESPONSE | jq '.references[] | select(.type == "external") | .name' | head -5

kill $SERVER_PID

echo "✅ Stage 4 Complete: Enhanced stdlib detection working"
```

### Stage 4 Success Criteria  
- ✅ More accurate standard library detection
- ✅ Proper classification of internal vs external references
- ✅ Performance maintained with caching
- ✅ Backward compatibility with existing detection method

---

## Stage 5: Testing, Optimization & Documentation
**Duration**: 2-3 days
**Goal**: Comprehensive validation and performance optimization

### Implementation Steps

#### 5.1 Comprehensive Test Suite (Day 1)

**Integration Tests**: `test/integration_test.go`
```go
func TestFullSystemIntegration(t *testing.T) {
    // Test complete workflow:
    // 1. Download module in isolation
    // 2. Analyze with go/packages  
    // 3. Verify all API endpoints work
    // 4. Check no host environment pollution
}

func TestPerformanceBenchmark(t *testing.T) {
    // Compare analysis speed: original vs hybrid
    // Memory usage comparison
    // Cache effectiveness
}

func TestEdgeCases(t *testing.T) {
    // Non-existent modules
    // Malformed version strings
    // Network failures
    // Permission issues
}
```

#### 5.2 Performance Optimization (Day 2)

```go
// Add analysis result caching
type AnalysisCache struct {
    packages map[string]*PackageInfo
    mutex    sync.RWMutex
    maxSize  int
}

func (h *HybridAnalyzer) AnalyzeVersionedPackageCached(moduleAtVersion, packagePath string) (*PackageInfo, error) {
    cacheKey := moduleAtVersion + "::" + packagePath
    
    // Check cache first
    if cached := h.cache.Get(cacheKey); cached != nil {
        return cached, nil
    }
    
    // Analyze and cache result
    result, err := h.AnalyzeVersionedPackage(moduleAtVersion, packagePath)
    if err == nil {
        h.cache.Set(cacheKey, result)
    }
    
    return result, err
}
```

#### 5.3 Documentation Update (Day 3)

Update `docs/API.md` with:
- New `/api/package-direct/` endpoint
- Isolation flag documentation
- Performance characteristics
- Migration notes

### Stage 5 Success Criteria
- ✅ Complete test coverage for all new functionality
- ✅ Performance benchmarks show improvement or parity
- ✅ Documentation reflects all changes
- ✅ Production-ready configuration examples

---

## Rollback Strategy

Each stage has a specific rollback approach:

**Stage 1**: Remove isolation flag, disable isolated environment
**Stage 2**: Set `GO_PACKAGES_ENABLED=false`, system falls back to original analyzer  
**Stage 3**: Remove new API endpoints, existing endpoints still work
**Stage 4-5**: Revert specific commits, core functionality unchanged

## Validation Checklist

After each stage:
- [ ] `go build` succeeds
- [ ] `go test ./...` passes
- [ ] Server starts without errors
- [ ] Existing API endpoints return expected responses
- [ ] New functionality works as designed
- [ ] No regression in performance
- [ ] Documentation is updated

## Success Metrics

**Functionality**: All existing APIs work unchanged
**Performance**: Analysis time ≤ current implementation  
**Quality**: Better symbol resolution and type information
**Isolation**: Zero host environment pollution in isolated mode
**Maintainability**: Reduced complexity in analyzer code

This implementation plan ensures each stage delivers a fully working system that can be deployed to production if needed, while building toward the complete go/packages integration.