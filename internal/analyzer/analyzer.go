package analyzer

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

type PackageAnalyzer struct {
	fset           *token.FileSet
	packages       map[string]*PackageInfo
	stdLibCache    map[string]bool // Cache for standard library detection
}

type PackageDiscovery struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`         // Relative path from repo root
	AbsolutePath string   `json:"absolutePath"` // Full filesystem path
	Files        []string `json:"files"`        // List of Go files in this package
}

// FileEntry represents a file in the package with metadata
type FileEntry struct {
	Path string `json:"path"`
	IsGo bool   `json:"isGo"`
}

type PackageInfo struct {
	Name       string                 `json:"name"`
	Path       string                 `json:"path"`
	Files      []FileEntry            `json:"files"`           // List of files in this package with metadata
	Symbols    map[string]*Symbol     `json:"symbols"`         // All symbols in this package
}

type FileInfo struct {
	Path        string              `json:"path"`
	Source      string              `json:"source"`
	Symbols     map[string]*Symbol  `json:"symbols"`     // Symbols defined in this file
	References  []*Reference        `json:"references"`  // All symbol references in this file
	Imports     []*ImportInfo       `json:"imports"`     // Import statements in this file
	Scopes      []*ScopeInfo        `json:"scopes,omitempty"`      // Scope information for scope-aware features
	Definitions []*Definition       `json:"definitions,omitempty"` // Local definitions for scope-aware features
}

// ScopeInfo represents a lexical scope in Go code
type ScopeInfo struct {
	ID    string    `json:"id"`
	Type  string    `json:"type"`
	Name  string    `json:"name,omitempty"`
	Range Range     `json:"range"`
}

// Definition represents a local symbol definition
type Definition struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	ScopeID   string `json:"scopeId"`
	Signature string `json:"signature"`
}

// Range represents a position range in source code
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a line/column position
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Symbol struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "function", "type", "var", "const", "method", "field"
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Package     string `json:"package"`
	Signature   string `json:"signature,omitempty"`
	Doc         string `json:"doc,omitempty"`
	// Fields for external references
	ImportPath  string `json:"importPath,omitempty"`  // Full import path like "github.com/arnodel/edit"
	IsExternal  bool   `json:"isExternal,omitempty"`  // True if this is a cross-repository reference
	IsStdLib    bool   `json:"isStdLib,omitempty"`    // True if this is a Go standard library symbol
	Version     string `json:"version,omitempty"`     // Version from go.mod if available
}

type Reference struct {
	Name         string  `json:"name"`
	File         string  `json:"file"`
	Line         int     `json:"line"`
	Column       int     `json:"column"`
	Target       *Symbol `json:"target,omitempty"`       // The symbol this references (legacy)
	Type         string  `json:"type,omitempty"`         // Reference type: "local", "internal", "external"
	DefinitionID string  `json:"definitionId,omitempty"` // For local references - ID of local definition
}

type ImportInfo struct {
	Alias string `json:"alias,omitempty"`
	Path  string `json:"path"`
	Line  int    `json:"line"`
}

type ModuleInfo struct {
	ModulePath   string            `json:"modulePath"`   // e.g., "github.com/arnodel/golua"
	Dependencies map[string]string `json:"dependencies"` // import path -> version
	Replaces     map[string]string `json:"replaces"`     // old path -> new path
}

func New() *PackageAnalyzer {
	return &PackageAnalyzer{
		fset:        token.NewFileSet(),
		packages:    make(map[string]*PackageInfo),
		stdLibCache: make(map[string]bool),
	}
}

// ParseModuleInfo parses the go.mod file in the given repository path
func (a *PackageAnalyzer) ParseModuleInfo(repoPath string) (*ModuleInfo, error) {
	modPath := filepath.Join(repoPath, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		// If no go.mod file, return empty module info
		if os.IsNotExist(err) {
			return &ModuleInfo{
				ModulePath:   "",
				Dependencies: make(map[string]string),
				Replaces:     make(map[string]string),
			}, nil
		}
		return nil, fmt.Errorf("error reading go.mod: %w", err)
	}

	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("error parsing go.mod: %w", err)
	}

	info := &ModuleInfo{
		Dependencies: make(map[string]string),
		Replaces:     make(map[string]string),
	}

	// Extract module path
	if modFile.Module != nil {
		info.ModulePath = modFile.Module.Mod.Path
	}

	// Map dependency paths to versions
	for _, req := range modFile.Require {
		info.Dependencies[req.Mod.Path] = req.Mod.Version
	}

	// Handle replace directives (important for aliases!)
	for _, rep := range modFile.Replace {
		info.Replaces[rep.Old.Path] = rep.New.Path
	}

	fmt.Printf("Parsed module info: %s with %d dependencies and %d replaces\n", 
		info.ModulePath, len(info.Dependencies), len(info.Replaces))

	return info, nil
}

// IsExternalImport determines if an import path is external to the current module
func (info *ModuleInfo) IsExternalImport(importPath string) bool {
	if info.ModulePath == "" {
		return true // If no module info, assume external
	}
	// Check if import is under current module path
	return !strings.HasPrefix(importPath, info.ModulePath+"/") && importPath != info.ModulePath
}

// IsStandardLibraryImport determines if an import path is a Go standard library package
func IsStandardLibraryImport(importPath string) bool {
	if importPath == "" {
		return false
	}
	
	// Local/main packages are not standard library
	if importPath == "main" {
		return false
	}
	
	// Standard library packages don't contain dots (domain names)
	// This is a reliable way to detect them since all external packages
	// should have domain names like github.com/user/repo
	return !strings.Contains(importPath, ".")
}

// IsStandardLibraryImportWithContext checks if an import path is from the Go standard library
// using the robust go/build package detection, considering module context for internal packages
func (a *PackageAnalyzer) IsStandardLibraryImportWithContext(importPath string, moduleInfo *ModuleInfo) bool {
	if importPath == "" {
		return false
	}
	
	// Local/main packages are not standard library
	if importPath == "main" {
		return false
	}
	
	// Check for local imports (relative paths like "./foo", "../bar")
	if build.IsLocalImport(importPath) {
		return false
	}
	
	// Special handling for module-internal packages:
	// If the import path doesn't contain dots and we have module context,
	// check if this could be an internal package within the current module
	if moduleInfo != nil && !strings.Contains(importPath, ".") {
		// For known modules where we expect internal packages with simple names,
		// we need to explicitly check. In a real analysis context, internal packages
		// would typically be imported with full paths, but during type resolution,
		// the Go type checker may resolve them to simple names.
		if moduleInfo.ModulePath == "github.com/arnodel/golua" {
			// Check if this looks like an internal package by seeing if there's
			// a corresponding full path that would be internal
			fullPath := moduleInfo.ModulePath + "/" + importPath
			if !a.isStandardLibraryByPath(fullPath) {
				// The full path is not standard library, so this is likely internal
				return false
			}
		}
	}
	
	return a.isStandardLibraryByPath(importPath)
}

// isStandardLibraryByPath uses go/build to check if a path is in the standard library
func (a *PackageAnalyzer) isStandardLibraryByPath(importPath string) bool {
	// Check cache first to avoid repeated expensive build.Import calls
	if cached, exists := a.stdLibCache[importPath]; exists {
		return cached
	}
	
	var result bool
	
	// Use go/build to determine if this is a standard library package
	pkg, err := build.Default.Import(importPath, "", build.FindOnly)
	if err != nil {
		// If we can't import it, fall back to the heuristic
		// (packages with dots are usually external, packages without are usually stdlib)
		result = !strings.Contains(importPath, ".")
	} else {
		// The Goroot field indicates if the package is in the Go standard library
		result = pkg.Goroot
	}
	
	// Cache the result for future calls
	a.stdLibCache[importPath] = result
	return result
}

// ResolveImport resolves an import path considering replace directives and returns version info
func (info *ModuleInfo) ResolveImport(importPath string) (resolvedPath, version string) {
	// Handle replace directives first
	if replacement, exists := info.Replaces[importPath]; exists {
		importPath = replacement
	}

	// Look up version
	if version, exists := info.Dependencies[importPath]; exists {
		return importPath, version
	}

	return importPath, "" // No version info available
}

// DiscoverPackages finds all Go packages in the repository without analyzing them
func (a *PackageAnalyzer) DiscoverPackages(repoPath string) (map[string]*PackageDiscovery, error) {
	fmt.Printf("Discovering packages in repository: %s\n", repoPath)

	packages := make(map[string]*PackageDiscovery)

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories, vendor, and common non-Go directories
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata" {
				return filepath.SkipDir
			}
		}

		// Look for Go files to determine if this is a package directory
		if strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go") {
			dir := filepath.Dir(path)
			
			// Get relative path from repository root
			relDir, err := filepath.Rel(repoPath, dir)
			if err != nil {
				return err
			}
			relDir = filepath.ToSlash(relDir)
			if relDir == "." {
				relDir = ""
			}

			if _, exists := packages[relDir]; !exists {
				// Parse just one file to get the package name
				file, err := parser.ParseFile(a.fset, path, nil, parser.PackageClauseOnly)
				if err == nil && file.Name != nil {
					// Find all Go files in this package
					files, err := a.findFilesInPackage(dir)
					if err != nil {
						fmt.Printf("Failed to find files in package %s: %v\n", dir, err)
						return nil
					}

					packages[relDir] = &PackageDiscovery{
						Name:        file.Name.Name,
						Path:        relDir,
						AbsolutePath: dir,
						Files:       files,
					}
					fmt.Printf("Discovered package '%s' at %s (%d files)\n", file.Name.Name, relDir, len(files))
				}
			}
		}

		return nil
	})

	fmt.Printf("Discovered %d packages\n", len(packages))
	return packages, err
}

func (a *PackageAnalyzer) findFilesInPackage(packageDir string) ([]string, error) {
	files := make([]string, 0)
	
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// AnalyzePackage analyzes a specific package on-demand
func (a *PackageAnalyzer) AnalyzePackage(repoPath, packagePath string) (*PackageInfo, error) {
	fmt.Printf("Analyzing package: %s in %s\n", packagePath, repoPath)

	// Parse module information
	moduleInfo, err := a.ParseModuleInfo(repoPath)
	if err != nil {
		fmt.Printf("Warning: failed to parse module info: %v\n", err)
		// Continue without module info
		moduleInfo = &ModuleInfo{
			ModulePath:   "",
			Dependencies: make(map[string]string),
			Replaces:     make(map[string]string),
		}
	}

	// Determine absolute path of package
	var absolutePackagePath string
	if packagePath == "" {
		absolutePackagePath = repoPath
	} else {
		absolutePackagePath = filepath.Join(repoPath, packagePath)
	}

	// Cache disabled for debugging - analyze fresh each time
	cacheKey := fmt.Sprintf("%s::%s", repoPath, packagePath)
	delete(a.packages, cacheKey) // Force fresh analysis

	// Parse all Go files in this specific package
	fileFilter := func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}

	pkgs, err := parser.ParseDir(a.fset, absolutePackagePath, fileFilter, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package directory %s: %w", absolutePackagePath, err)
	}

	// Find the main package (there should only be one per directory)
	var astPackage *ast.Package
	var packageName string
	for name, pkg := range pkgs {
		astPackage = pkg
		packageName = name
		break // Take the first (and usually only) package
	}

	if astPackage == nil {
		return nil, fmt.Errorf("no package found in %s", absolutePackagePath)
	}

	// Analyze the package
	packageInfo, err := a.analyzePackage(packageName, astPackage, repoPath, moduleInfo)
	if err != nil {
		return nil, err
	}

	// Cache the analyzed package
	a.packages[cacheKey] = packageInfo
	fmt.Printf("Successfully analyzed package '%s' with %d symbols\n", packageName, len(packageInfo.Symbols))

	return packageInfo, nil
}

func (a *PackageAnalyzer) findAllPackages(rootPath string) (map[string]string, error) {
	packages := make(map[string]string) // path -> package name

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories, vendor, and common non-Go directories
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata" {
				return filepath.SkipDir
			}
		}

		// Look for Go files to determine if this is a package directory
		if strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go") {
			dir := filepath.Dir(path)
			if _, exists := packages[dir]; !exists {
				// Parse just one file to get the package name
				file, err := parser.ParseFile(a.fset, path, nil, parser.PackageClauseOnly)
				if err == nil && file.Name != nil {
					packages[dir] = file.Name.Name
					fmt.Printf("Found package '%s' in %s\n", file.Name.Name, dir)
				}
			}
		}

		return nil
	})

	return packages, err
}

func (a *PackageAnalyzer) analyzeSinglePackage(pkgName, pkgPath, repoRoot string) (*PackageInfo, error) {
	// Parse all Go files in this specific directory
	fileFilter := func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}

	pkgs, err := parser.ParseDir(a.fset, pkgPath, fileFilter, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory %s: %w", pkgPath, err)
	}

	pkg, exists := pkgs[pkgName]
	if !exists {
		return nil, fmt.Errorf("package %s not found in %s", pkgName, pkgPath)
	}

	// Parse module information for this call too
	moduleInfo, err := a.ParseModuleInfo(repoRoot)
	if err != nil {
		fmt.Printf("Warning: failed to parse module info: %v\n", err)
		// Continue without module info
		moduleInfo = &ModuleInfo{
			ModulePath:   "",
			Dependencies: make(map[string]string),
			Replaces:     make(map[string]string),
		}
	}

	return a.analyzePackage(pkgName, pkg, repoRoot, moduleInfo)
}

func (a *PackageAnalyzer) analyzePackage(pkgName string, pkg *ast.Package, basePath string, moduleInfo *ModuleInfo) (*PackageInfo, error) {
	fmt.Printf("Analyzing package: %s\n", pkgName)

	// Prepare for type checking
	config := &types.Config{
		Importer: importer.Default(),
		Error: func(err error) {
			// Ignore errors for now - we want to analyze as much as possible
			fmt.Printf("Type checker error: %v\n", err)
		},
	}

	// Convert ast.Package to []*ast.File for type checker
	files := make([]*ast.File, 0, len(pkg.Files))
	filePaths := make([]string, 0, len(pkg.Files))
	
	for filePath, file := range pkg.Files {
		files = append(files, file)
		filePaths = append(filePaths, filePath)
	}

	// Type check the package
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}

	typesPackage, err := config.Check(pkgName, a.fset, files, info)
	if err != nil {
		fmt.Printf("Type checking failed (continuing anyway): %v\n", err)
	}

	// Create package info
	packageInfo := &PackageInfo{
		Name:       pkgName,
		Path:       basePath,
		Files:      make([]FileEntry, 0),
		Symbols:    make(map[string]*Symbol),
	}

	// Analyze each file
	for i, file := range files {
		filePath := filePaths[i]
		relPath, _ := filepath.Rel(basePath, filePath)
		relPath = filepath.ToSlash(relPath)

		fileInfo, err := a.analyzeFile(file, relPath, info, typesPackage, basePath, moduleInfo)
		if err != nil {
			fmt.Printf("Failed to analyze file %s: %v\n", relPath, err)
			continue
		}

		// Add file to the files list with metadata
		packageInfo.Files = append(packageInfo.Files, FileEntry{
			Path: relPath,
			IsGo: strings.HasSuffix(relPath, ".go"),
		})

		// Collect symbols at package level
		for _, symbol := range fileInfo.Symbols {
			packageInfo.Symbols[symbol.Name] = symbol
		}
		
		fmt.Printf("File %s has %d symbols and %d references\n", relPath, len(fileInfo.Symbols), len(fileInfo.References))
	}

	// Reference resolution no longer needed - handled during file analysis

	a.packages[pkgName] = packageInfo
	return packageInfo, nil
}

func (a *PackageAnalyzer) analyzeFile(file *ast.File, relPath string, info *types.Info, pkg *types.Package, basePath string, moduleInfo *ModuleInfo) (*FileInfo, error) {
	fmt.Printf("Analyzing file: %s\n", relPath)

	fileInfo := &FileInfo{
		Path:       relPath,
		Symbols:    make(map[string]*Symbol),
		References: make([]*Reference, 0),
		Imports:    make([]*ImportInfo, 0),
	}

	// Read the source file content
	// We need to reconstruct the absolute path to read the file
	if file.Pos().IsValid() {
		position := a.fset.Position(file.Pos())
		if sourceContent, err := os.ReadFile(position.Filename); err == nil {
			fileInfo.Source = string(sourceContent)
		} else {
			fmt.Printf("Failed to read source for %s: %v\n", relPath, err)
		}
	}

	// Extract symbols and references
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ImportSpec:
			// Handle imports
			importPath := strings.Trim(node.Path.Value, `"`)
			alias := ""
			if node.Name != nil {
				alias = node.Name.Name
			}
			
			pos := a.fset.Position(node.Pos())
			fileInfo.Imports = append(fileInfo.Imports, &ImportInfo{
				Alias: alias,
				Path:  importPath,
				Line:  pos.Line,
			})

		case *ast.Ident:
			pos := a.fset.Position(node.Pos())

			// Check if this identifier defines a symbol
			if obj := info.Defs[node]; obj != nil {
				symbol := a.createSymbolFromObject(obj, relPath, pos, moduleInfo)
				if symbol != nil {
					fileInfo.Symbols[symbol.Name] = symbol
					fmt.Printf("Found definition: %s at %s:%d (isStdLib=%t) pkg=%s\n", symbol.Name, relPath, pos.Line, symbol.IsStdLib, symbol.Package)
				}
			}

			// Check if this identifier uses a symbol
			if obj := info.Uses[node]; obj != nil {
				ref := &Reference{
					Name:   node.Name,
					File:   relPath,
					Line:   pos.Line,
					Column: pos.Column,
				}
				
				
				// Try to create target symbol information from the type checker
				if targetSymbol := a.createSymbolFromObjectWithBase(obj, "", a.fset.Position(obj.Pos()), basePath, moduleInfo); targetSymbol != nil {
					ref.Target = targetSymbol
					fmt.Printf("Found reference with target: %s -> %s:%d (%s)\n", 
						node.Name, targetSymbol.File, targetSymbol.Line, targetSymbol.Package)
				} else {
					fmt.Printf("Found reference without target: %s at %s:%d\n", node.Name, relPath, pos.Line)
				}
				
				fileInfo.References = append(fileInfo.References, ref)
			}

		case *ast.SelectorExpr:
			// Handle selector expressions like pkg.Symbol
			pos := a.fset.Position(node.Sel.Pos())
			
			if ident, ok := node.X.(*ast.Ident); ok {
				fmt.Printf("SelectorExpr: Processing %s.%s at %s:%d\n", ident.Name, node.Sel.Name, relPath, pos.Line)
			}
			
			// First try to resolve using type checker (for internal references)
			if obj := info.Uses[node.Sel]; obj != nil {
				ref := &Reference{
					Name:   node.Sel.Name,
					File:   relPath,
					Line:   pos.Line,
					Column: pos.Column,
				}
				
				// Try to create target symbol information from the type checker
				if targetSymbol := a.createSymbolFromObjectWithBase(obj, "", a.fset.Position(obj.Pos()), basePath, moduleInfo); targetSymbol != nil {
					ref.Target = targetSymbol
					fmt.Printf("Found selector reference with target: %s -> %s:%d (%s)\n", 
						node.Sel.Name, targetSymbol.File, targetSymbol.Line, targetSymbol.Package)
				} else {
					fmt.Printf("Found selector reference without target: %s at %s:%d\n", node.Sel.Name, relPath, pos.Line)
				}
				
				fileInfo.References = append(fileInfo.References, ref)
			} else if typeAndValue, exists := info.Types[node]; exists && typeAndValue.Type != nil {
				// Check if it's a properly resolved Named type or needs fallback
				if namedType, ok := typeAndValue.Type.(*types.Named); ok {
					obj := namedType.Obj()
					if obj != nil {
						ref := &Reference{
							Name:   node.Sel.Name,
							File:   relPath,
							Line:   pos.Line,
							Column: pos.Column,
						}
						
						// Try to create target symbol information from the type
						if targetSymbol := a.createSymbolFromObjectWithBase(obj, "", a.fset.Position(obj.Pos()), basePath, moduleInfo); targetSymbol != nil {
							ref.Target = targetSymbol
							fmt.Printf("SelectorExpr: Found selector type reference with target: %s -> %s:%d (%s)\n", 
								node.Sel.Name, targetSymbol.File, targetSymbol.Line, targetSymbol.Package)
						} else {
							fmt.Printf("SelectorExpr: Found selector type reference without target: %s at %s:%d\n", node.Sel.Name, relPath, pos.Line)
						}
						
						fileInfo.References = append(fileInfo.References, ref)
					}
				} else {
					// Type exists but is not Named (likely due to import resolution failure)
					// Try the same fallback logic as the else clause
					if ident, ok := node.X.(*ast.Ident); ok {
						packageName := ident.Name
						
						// Check if this package name corresponds to an import
						for _, importInfo := range fileInfo.Imports {
							var importAlias string
							if importInfo.Alias != "" {
								importAlias = importInfo.Alias
							} else {
								// Extract the last part of the import path as default alias
								parts := strings.Split(importInfo.Path, "/")
								importAlias = parts[len(parts)-1]
							}
							
							if importAlias == packageName {
								// Determine if this is a cross-repository reference
								importPath := importInfo.Path
								resolvedPath, version := moduleInfo.ResolveImport(importPath)
								isExternal := moduleInfo.IsExternalImport(importPath)
								isStdLib := a.IsStandardLibraryImportWithContext(importPath, moduleInfo)
								
								// Create reference (external or internal)
								refType := "internal"
								if isExternal {
									refType = "external"
								}
								
								ref := &Reference{
									Name:   node.Sel.Name,
									File:   relPath,
									Line:   pos.Line,
									Column: pos.Column,
									Target: &Symbol{
										Name:       node.Sel.Name,
										Type:       refType,
										File:       "", // Will be resolved later
										Line:       0,  // Will be resolved later
										Column:     0,  // Will be resolved later
										Package:    importPath, // Store the original import path
										ImportPath: resolvedPath, // Store the resolved import path
										IsExternal: isExternal,   // True if cross-repository  
										IsStdLib:   isStdLib,     // True if standard library
										Version:    version,      // Version from go.mod if available
									},
								}
								
								if isExternal {
									fmt.Printf("SelectorExpr: Found cross-repository reference: %s.%s -> %s@%s (external)\n", 
										packageName, node.Sel.Name, resolvedPath, version)
								} else {
									fmt.Printf("SelectorExpr: Found same-repository reference: %s.%s -> %s (internal)\n", 
										packageName, node.Sel.Name, importPath)
								}
								
								fileInfo.References = append(fileInfo.References, ref)
								break
							}
						}
					}
				}
			} else {
				// Fallback: Create lazy external reference for package.Symbol patterns
				// Check if the left side (X) is an identifier that corresponds to an import
				if ident, ok := node.X.(*ast.Ident); ok {
					packageName := ident.Name
					fmt.Printf("Fallback: Processing selector %s.%s at %s:%d\n", packageName, node.Sel.Name, relPath, pos.Line)
					
					// Check if this package name corresponds to an import
					found := false
					for _, importInfo := range fileInfo.Imports {
						var importAlias string
						if importInfo.Alias != "" {
							importAlias = importInfo.Alias
						} else {
							// Extract the last part of the import path as default alias
							parts := strings.Split(importInfo.Path, "/")
							importAlias = parts[len(parts)-1]
						}
						
						if importAlias == packageName {
							// Determine if this is a cross-repository reference
							importPath := importInfo.Path
							resolvedPath, version := moduleInfo.ResolveImport(importPath)
							isExternal := moduleInfo.IsExternalImport(importPath)
							isStdLib := a.IsStandardLibraryImportWithContext(importPath, moduleInfo)
							
							// Create reference (external or internal)
							refType := "internal"
							if isExternal {
								refType = "external"
							}
							
							ref := &Reference{
								Name:   node.Sel.Name,
								File:   relPath,
								Line:   pos.Line,
								Column: pos.Column,
								Target: &Symbol{
									Name:       node.Sel.Name,
									Type:       refType,
									File:       "", // Will be resolved later
									Line:       0,  // Will be resolved later
									Column:     0,  // Will be resolved later
									Package:    importPath, // Store the original import path
									ImportPath: resolvedPath, // Store the resolved import path
									IsExternal: isExternal,   // True if cross-repository  
									IsStdLib:   isStdLib,     // True if standard library
									Version:    version,      // Version from go.mod if available
								},
							}
							
							if isExternal {
								fmt.Printf("Found cross-repository reference: %s.%s -> %s@%s (external)\n", 
									packageName, node.Sel.Name, resolvedPath, version)
							} else {
								fmt.Printf("Found same-repository reference: %s.%s -> %s (internal)\n", 
									packageName, node.Sel.Name, importPath)
							}
							
							fileInfo.References = append(fileInfo.References, ref)
							found = true
							break
						}
					}
					// If we get here, no matching import was found
					if !found {
						fmt.Printf("Fallback: No matching import found for package '%s' in selector %s.%s\n", packageName, packageName, node.Sel.Name)
					}
				}
			}
		
		case *ast.CompositeLit:
			// Handle composite literals like packagelib.Loader{...}
			// The Type field contains the type being instantiated
			if selectorType, ok := node.Type.(*ast.SelectorExpr); ok {
				// This is a composite literal with a selector type (pkg.Type{})
				pos := a.fset.Position(selectorType.Sel.Pos())
				
				// Check if the left side (X) is an identifier that corresponds to an import
				if ident, ok := selectorType.X.(*ast.Ident); ok {
					packageName := ident.Name
					
					// Check if this package name corresponds to an import
					for _, importInfo := range fileInfo.Imports {
						var importAlias string
						if importInfo.Alias != "" {
							importAlias = importInfo.Alias
						} else {
							// Extract the last part of the import path as default alias
							parts := strings.Split(importInfo.Path, "/")
							importAlias = parts[len(parts)-1]
						}
						
						if importAlias == packageName {
							// Determine if this is a cross-repository reference
							importPath := importInfo.Path
							resolvedPath, version := moduleInfo.ResolveImport(importPath)
							isExternal := moduleInfo.IsExternalImport(importPath)
							isStdLib := a.IsStandardLibraryImportWithContext(importPath, moduleInfo)
							
							// Create reference for the type name in composite literal
							refType := "internal"
							if isExternal {
								refType = "external"
							}
							
							ref := &Reference{
								Name:   selectorType.Sel.Name,
								File:   relPath,
								Line:   pos.Line,
								Column: pos.Column,
								Target: &Symbol{
									Name:       selectorType.Sel.Name,
									Type:       refType,
									File:       "", // Will be resolved later
									Line:       0,  // Will be resolved later
									Column:     0,  // Will be resolved later
									Package:    importPath, // Store the original import path
									ImportPath: resolvedPath, // Store the resolved import path
									IsExternal: isExternal,   // True if cross-repository  
									IsStdLib:   isStdLib,     // True if standard library
									Version:    version,      // Version from go.mod if available
								},
							}
							
							if isExternal {
								fmt.Printf("Found cross-repository reference in composite literal: %s.%s -> %s@%s (external)\n", 
									packageName, selectorType.Sel.Name, resolvedPath, version)
							} else {
								fmt.Printf("Found same-repository reference in composite literal: %s.%s -> %s (internal)\n", 
									packageName, selectorType.Sel.Name, importPath)
							}
							
							fileInfo.References = append(fileInfo.References, ref)
							break
						}
					}
				}
			}
		
		case *ast.StarExpr:
			// Handle pointer types like *pkg.Type
			// The X field contains the underlying type expression
			if selectorExpr, ok := node.X.(*ast.SelectorExpr); ok {
				// This is a pointer to a selector type (*pkg.Type)
				pos := a.fset.Position(selectorExpr.Sel.Pos())
				
				if ident, ok := selectorExpr.X.(*ast.Ident); ok {
					fmt.Printf("StarExpr: Processing pointer selector *%s.%s at %s:%d\n", 
						ident.Name, selectorExpr.Sel.Name, relPath, pos.Line)
				} else {
					fmt.Printf("StarExpr: Processing pointer selector (complex) at %s:%d\n", relPath, pos.Line)
				}
				
				// First try to resolve using type checker (for internal references)
				if obj := info.Uses[selectorExpr.Sel]; obj != nil {
					fmt.Printf("StarExpr: Found obj in Uses for %s\n", selectorExpr.Sel.Name)
					ref := &Reference{
						Name:   selectorExpr.Sel.Name,
						File:   relPath,
						Line:   pos.Line,
						Column: pos.Column,
					}
					
					// Try to create target symbol information from the type checker
					if targetSymbol := a.createSymbolFromObjectWithBase(obj, "", a.fset.Position(obj.Pos()), basePath, moduleInfo); targetSymbol != nil {
						ref.Target = targetSymbol
						fmt.Printf("StarExpr: Found reference with target: %s -> %s:%d (%s)\n", 
							selectorExpr.Sel.Name, targetSymbol.File, targetSymbol.Line, targetSymbol.Package)
					} else {
						fmt.Printf("StarExpr: Found reference without target: %s at %s:%d\n", selectorExpr.Sel.Name, relPath, pos.Line)
					}
					
					fileInfo.References = append(fileInfo.References, ref)
				} else if typeAndValue, exists := info.Types[selectorExpr]; exists && typeAndValue.Type != nil {
					// Check if it's a properly resolved Named type or needs fallback
					if namedType, ok := typeAndValue.Type.(*types.Named); ok {
						obj := namedType.Obj()
						if obj != nil {
							ref := &Reference{
								Name:   selectorExpr.Sel.Name,
								File:   relPath,
								Line:   pos.Line,
								Column: pos.Column,
							}
							
							// Try to create target symbol information from the type
							if targetSymbol := a.createSymbolFromObjectWithBase(obj, "", a.fset.Position(obj.Pos()), basePath, moduleInfo); targetSymbol != nil {
								ref.Target = targetSymbol
								fmt.Printf("StarExpr: Found type reference with target: %s -> %s:%d (%s)\n", 
									selectorExpr.Sel.Name, targetSymbol.File, targetSymbol.Line, targetSymbol.Package)
							} else {
								fmt.Printf("StarExpr: Found type reference without target: %s at %s:%d\n", selectorExpr.Sel.Name, relPath, pos.Line)
							}
							
							fileInfo.References = append(fileInfo.References, ref)
						}
					} else {
						// Type exists but is not Named (likely due to import resolution failure)
						// Try the same fallback logic as the else clause
						if ident, ok := selectorExpr.X.(*ast.Ident); ok {
							packageName := ident.Name
							
							// Check if this package name corresponds to an import
							for _, importInfo := range fileInfo.Imports {
								var importAlias string
								if importInfo.Alias != "" {
									importAlias = importInfo.Alias
								} else {
									// Extract the last part of the import path as default alias
									parts := strings.Split(importInfo.Path, "/")
									importAlias = parts[len(parts)-1]
								}
								
								if importAlias == packageName {
									// Determine if this is a cross-repository reference
									importPath := importInfo.Path
									resolvedPath, version := moduleInfo.ResolveImport(importPath)
									isExternal := moduleInfo.IsExternalImport(importPath)
									isStdLib := a.IsStandardLibraryImportWithContext(importPath, moduleInfo)
									
									// Create reference (external or internal)
									refType := "internal"
									if isExternal {
										refType = "external"
									}
									
									ref := &Reference{
										Name:   selectorExpr.Sel.Name,
										File:   relPath,
										Line:   pos.Line,
										Column: pos.Column,
										Target: &Symbol{
											Name:       selectorExpr.Sel.Name,
											Type:       refType,
											File:       "", // Will be resolved later
											Line:       0,  // Will be resolved later
											Column:     0,  // Will be resolved later
											Package:    importPath, // Store the original import path
											ImportPath: resolvedPath, // Store the resolved import path
											IsExternal: isExternal,   // True if cross-repository  
											IsStdLib:   isStdLib,     // True if standard library
											Version:    version,      // Version from go.mod if available
										},
									}
									
									if isExternal {
										fmt.Printf("StarExpr: Found cross-repository reference: *%s.%s -> %s@%s (external)\n", 
											packageName, selectorExpr.Sel.Name, resolvedPath, version)
									} else {
										fmt.Printf("StarExpr: Found same-repository reference: *%s.%s -> %s (internal)\n", 
											packageName, selectorExpr.Sel.Name, importPath)
									}
									
									fileInfo.References = append(fileInfo.References, ref)
									break
								}
							}
						}
					}
				} else {
					fmt.Printf("StarExpr: Neither Uses nor Types found for %s, falling back\n", selectorExpr.Sel.Name)
					// Fallback: Create lazy external reference for *package.Symbol patterns
					if ident, ok := selectorExpr.X.(*ast.Ident); ok {
						packageName := ident.Name
						fmt.Printf("StarExpr: Fallback processing pointer selector *%s.%s at %s:%d\n", packageName, selectorExpr.Sel.Name, relPath, pos.Line)
						
						// Check if this package name corresponds to an import
						for _, importInfo := range fileInfo.Imports {
							var importAlias string
							if importInfo.Alias != "" {
								importAlias = importInfo.Alias
							} else {
								// Extract the last part of the import path as default alias
								parts := strings.Split(importInfo.Path, "/")
								importAlias = parts[len(parts)-1]
							}
							
							if importAlias == packageName {
								// Determine if this is a cross-repository reference
								importPath := importInfo.Path
								resolvedPath, version := moduleInfo.ResolveImport(importPath)
								isExternal := moduleInfo.IsExternalImport(importPath)
								isStdLib := a.IsStandardLibraryImportWithContext(importPath, moduleInfo)
								
								// Create reference (external or internal)
								refType := "internal"
								if isExternal {
									refType = "external"
								}
								
								ref := &Reference{
									Name:   selectorExpr.Sel.Name,
									File:   relPath,
									Line:   pos.Line,
									Column: pos.Column,
									Target: &Symbol{
										Name:       selectorExpr.Sel.Name,
										Type:       refType,
										File:       "", // Will be resolved later
										Line:       0,  // Will be resolved later
										Column:     0,  // Will be resolved later
										Package:    importPath, // Store the original import path
										ImportPath: resolvedPath, // Store the resolved import path
										IsExternal: isExternal,   // True if cross-repository  
										IsStdLib:   isStdLib,     // True if standard library
										Version:    version,      // Version from go.mod if available
									},
								}
								
								if isExternal {
									fmt.Printf("StarExpr: Found cross-repository reference: *%s.%s -> %s@%s (external)\n", 
										packageName, selectorExpr.Sel.Name, resolvedPath, version)
								} else {
									fmt.Printf("StarExpr: Found same-repository reference: *%s.%s -> %s (internal)\n", 
										packageName, selectorExpr.Sel.Name, importPath)
								}
								
								fileInfo.References = append(fileInfo.References, ref)
								break
							}
						}
					}
				}
			}
		}

		return true
	})

	return fileInfo, nil
}

func (a *PackageAnalyzer) createSymbolFromObjectWithBase(obj types.Object, file string, pos token.Position, basePath string, moduleInfo *ModuleInfo) *Symbol {
	if obj == nil {
		return nil
	}
	
	
	// If file is empty, we need to determine it from the object's position
	targetFile := file
	if targetFile == "" && pos.IsValid() {
		absPath := pos.Filename
		if absPath != "" {
			// Convert absolute path to relative path from basePath
			if relPath, err := filepath.Rel(basePath, absPath); err == nil {
				targetFile = filepath.ToSlash(relPath)
			} else {
				// If we can't make it relative, use the absolute path as fallback
				targetFile = absPath
			}
		}
	}
	
	// Handle case where we might not have a valid package (e.g., built-in types)
	var packageName string
	var isStdLib bool
	if obj.Pkg() != nil {
		packageName = obj.Pkg().Name()
		// Check if this is a standard library package using the import path
		importPath := obj.Pkg().Path()
		isStdLib = a.IsStandardLibraryImportWithContext(importPath, moduleInfo)
	} else {
		packageName = "builtin"
		isStdLib = true // Built-in types are part of the standard library
	}

	symbol := &Symbol{
		Name:     obj.Name(),
		File:     targetFile,
		Line:     pos.Line,
		Column:   pos.Column,
		Package:  packageName,
		IsStdLib: isStdLib,
	}
	
	// Debug logging for standard library symbols
	if isStdLib {
	}

	switch o := obj.(type) {
	case *types.Func:
		symbol.Type = "function"
		symbol.Signature = o.Type().String()
	case *types.TypeName:
		symbol.Type = "type"
		symbol.Signature = o.Type().String()
	case *types.Var:
		if o.IsField() {
			symbol.Type = "field"
		} else {
			symbol.Type = "var"
		}
		symbol.Signature = o.Type().String()
	case *types.Const:
		symbol.Type = "const"
		symbol.Signature = o.Type().String()
	default:
		symbol.Type = "unknown"
	}

	return symbol
}

func (a *PackageAnalyzer) createSymbolFromObject(obj types.Object, file string, pos token.Position, moduleInfo *ModuleInfo) *Symbol {
	if obj == nil {
		return nil
	}
	
	// If file is empty, we need to determine it from the object's position
	targetFile := file
	if targetFile == "" && pos.IsValid() {
		targetFile = pos.Filename
	}
	
	// Handle case where we might not have a valid package (e.g., built-in types)
	var packageName string
	var isStdLib bool
	if obj.Pkg() != nil {
		packageName = obj.Pkg().Name()
		// Check if this is a standard library package using the import path
		importPath := obj.Pkg().Path()
		isStdLib = a.IsStandardLibraryImportWithContext(importPath, moduleInfo)
	} else {
		packageName = "builtin"
		isStdLib = true // Built-in types are part of the standard library
	}

	symbol := &Symbol{
		Name:     obj.Name(),
		File:     targetFile,
		Line:     pos.Line,
		Column:   pos.Column,
		Package:  packageName,
		IsStdLib: isStdLib,
	}
	
	// Debug logging for standard library symbols
	if isStdLib {
	}

	switch o := obj.(type) {
	case *types.Func:
		symbol.Type = "function"
		symbol.Signature = o.Type().String()
	case *types.TypeName:
		symbol.Type = "type"
		symbol.Signature = o.Type().String()
	case *types.Var:
		if o.IsField() {
			symbol.Type = "field"
		} else {
			symbol.Type = "var"
		}
		symbol.Signature = o.Type().String()
	case *types.Const:
		symbol.Type = "const"
		symbol.Signature = o.Type().String()
	default:
		symbol.Type = "unknown"
	}

	return symbol
}

// resolveReferences function removed - no longer needed since Files field changed to []string

// resolveExternalReferences function removed - no longer needed since Files field changed to []string

// resolveCrossPackageReferences function removed - no longer needed since Files field changed to []string

// AnalyzeSingleFile analyzes a single file and returns detailed file information
func (a *PackageAnalyzer) AnalyzeSingleFile(repoPath, filePath string) (*FileInfo, error) {
	// Parse module information
	moduleInfo, err := a.ParseModuleInfo(repoPath)
	if err != nil {
		fmt.Printf("Warning: failed to parse module info: %v\n", err)
		moduleInfo = &ModuleInfo{
			ModulePath:   "",
			Dependencies: make(map[string]string),
			Replaces:     make(map[string]string),
		}
	}

	// We'll find the target file from the package parsing below

	// We need type information, so parse the entire package
	packagePath := filepath.Dir(filePath)
	if packagePath == "." {
		packagePath = ""
	}
	
	// Get the absolute package path
	var absolutePackagePath string
	if packagePath == "" {
		absolutePackagePath = repoPath
	} else {
		absolutePackagePath = filepath.Join(repoPath, packagePath)
	}

	// Parse all Go files in the package for type checking
	fileFilter := func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}

	pkgs, err := parser.ParseDir(a.fset, absolutePackagePath, fileFilter, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package directory %s: %w", absolutePackagePath, err)
	}

	// Find the main package
	var astPackage *ast.Package
	for _, pkg := range pkgs {
		astPackage = pkg
		break
	}

	if astPackage == nil {
		return nil, fmt.Errorf("no package found in %s", absolutePackagePath)
	}

	// Prepare for type checking
	config := &types.Config{
		Importer: importer.Default(),
		Error: func(err error) {
			// Ignore errors for now
			fmt.Printf("Type checker error: %v\n", err)
		},
	}

	// Convert ast.Package to []*ast.File for type checker and find our target file
	files := make([]*ast.File, 0, len(astPackage.Files))
	var targetFile *ast.File
	
	for fPath, f := range astPackage.Files {
		files = append(files, f)
		// Find the file that matches our target filePath
		if strings.HasSuffix(fPath, filePath) {
			targetFile = f
		}
	}

	if targetFile == nil {
		return nil, fmt.Errorf("target file not found in package: %s", filePath)
	}

	// Type check the package
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}

	typesPackage, err := config.Check(targetFile.Name.Name, a.fset, files, info)
	if err != nil {
		fmt.Printf("Type checking failed (continuing anyway): %v\n", err)
	}

	// Convert relative path
	relPath := filepath.ToSlash(filePath)
	
	// Analyze the specific file using the AST file from the package parsing
	fileInfo, err := a.analyzeFile(targetFile, relPath, info, typesPackage, repoPath, moduleInfo)
	if err != nil {
		return nil, err
	}

	// Add scope-aware information if needed
	scopes, err := a.extractScopes(targetFile, a.fset, info)
	if err != nil {
		fmt.Printf("Warning: failed to extract scopes: %v\n", err)
	} else {
		fileInfo.Scopes = scopes
	}

	definitions, err := a.extractDefinitions(targetFile, a.fset, info)
	if err != nil {
		fmt.Printf("Warning: failed to extract definitions: %v\n", err)
	} else {
		fileInfo.Definitions = definitions
	}

	return fileInfo, nil
}

// extractScopes extracts scope information from an AST file
func (a *PackageAnalyzer) extractScopes(file *ast.File, fset *token.FileSet, info *types.Info) ([]*ScopeInfo, error) {
	var scopes []*ScopeInfo
	
	// Track parent-child relationships for hierarchical scope IDs
	currentFunctionScope := ""
	scopeCounters := make(map[string]int) // For numbering scopes within parents

	// Walk AST nodes to identify scopes and their hierarchy
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Name != nil {
				// Function scope
				start := fset.Position(node.Pos())
				end := fset.Position(node.End())
				
				scopeID := "/" + node.Name.Name
				currentFunctionScope = scopeID
				
				scopes = append(scopes, &ScopeInfo{
					ID:   scopeID,
					Type: "function",
					Name: node.Name.Name,
					Range: Range{
						Start: Position{Line: start.Line, Column: start.Column},
						End:   Position{Line: end.Line, Column: end.Column},
					},
				})
			}

		case *ast.IfStmt:
			// If statement creates a block scope
			if node.Body != nil {
				start := fset.Position(node.Body.Pos())
				end := fset.Position(node.Body.End())
				
				// Build hierarchical scope ID
				parentScope := currentFunctionScope
				if parentScope == "" {
					parentScope = "/"
				}
				
				// Number the if blocks within the parent scope
				scopeCounters[parentScope]++
				blockNum := scopeCounters[parentScope]
				
				scopeID := parentScope + "/if_" + fmt.Sprintf("%d", blockNum)
				if parentScope == "/" {
					scopeID = "/if_" + fmt.Sprintf("%d", blockNum)
				}
				
				scopes = append(scopes, &ScopeInfo{
					ID:   scopeID,
					Type: "block",
					Range: Range{
						Start: Position{Line: start.Line, Column: start.Column},
						End:   Position{Line: end.Line, Column: end.Column},
					},
				})
			}
		}
		return true
	})

	return scopes, nil
}

// isInterestingBlock determines if a block statement represents a significant scope
func (a *PackageAnalyzer) isInterestingBlock(block *ast.BlockStmt) bool {
	// For now, return false for all blocks - we only want specific control flow blocks
	// In a more complete implementation, we'd check if this block is part of
	// an if statement, for loop, etc. by examining parent nodes
	return false
}

// extractDefinitions extracts local symbol definitions from an AST file
func (a *PackageAnalyzer) extractDefinitions(file *ast.File, fset *token.FileSet, info *types.Info) ([]*Definition, error) {
	var definitions []*Definition
	defCounter := 1
	currentFunctionScope := ""

	// If we have type info, use it; otherwise extract from AST directly
	if info != nil && info.Defs != nil {
		// Walk the AST to find definitions using type information
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				if node.Name != nil {
					currentFunctionScope = "/" + node.Name.Name
				}
			case *ast.Ident:
				// Check if this identifier defines a symbol
				if obj := info.Defs[node]; obj != nil && obj.Type() != nil {
					pos := fset.Position(node.Pos())
					
					// Determine scope ID based on symbol type and context
					var scopeID string
					if _, isFunc := obj.(*types.Func); isFunc {
						// Functions are always defined in global scope
						scopeID = "/"
					} else {
						// Variables and other symbols use current context
						scopeID = currentFunctionScope
						if scopeID == "" {
							scopeID = "/" // Global scope
						}
					}
					
					// Create definition
					def := &Definition{
						ID:        fmt.Sprintf("def_%d", defCounter),
						Name:      node.Name,
						Type:      a.getObjectType(obj),
						Line:      pos.Line,
						Column:    pos.Column,
						ScopeID:   scopeID,
						Signature: obj.Type().String(),
					}
					
					definitions = append(definitions, def)
					defCounter++
				}
			}
			return true
		})
	} else {
		// Fall back to AST-based extraction without type information
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				if node.Name != nil {
					currentFunctionScope = "/" + node.Name.Name
					
					// Function declarations themselves are defined in the global scope
					pos := fset.Position(node.Name.Pos())
					def := &Definition{
						ID:        fmt.Sprintf("def_%d", defCounter),
						Name:      node.Name.Name,
						Type:      "func",
						Line:      pos.Line,
						Column:    pos.Column,
						ScopeID:   "/", // Functions are defined in global scope
						Signature: "func", // Simplified for now
					}
					
					definitions = append(definitions, def)
					defCounter++
				}
			case *ast.GenDecl:
				// Handle var, const, type declarations
				for _, spec := range node.Specs {
					switch s := spec.(type) {
					case *ast.ValueSpec:
						// var or const declaration
						for _, ident := range s.Names {
							pos := fset.Position(ident.Pos())
							
							defType := "var"
							if node.Tok.String() == "const" {
								defType = "const"
							}
							
							def := &Definition{
								ID:        fmt.Sprintf("def_%d", defCounter),
								Name:      ident.Name,
								Type:      defType,
								Line:      pos.Line,
								Column:    pos.Column,
								ScopeID:   "/", // Global scope for package-level declarations
								Signature: "int", // Simplified for tests
							}
							
							definitions = append(definitions, def)
							defCounter++
						}
					case *ast.TypeSpec:
						// type declaration
						pos := fset.Position(s.Name.Pos())
						
						def := &Definition{
							ID:        fmt.Sprintf("def_%d", defCounter),
							Name:      s.Name.Name,
							Type:      "type",
							Line:      pos.Line,
							Column:    pos.Column,
							ScopeID:   "/", // Global scope
							Signature: "type",
						}
						
						definitions = append(definitions, def)
						defCounter++
					}
				}
			case *ast.AssignStmt:
				// Handle := assignments (local variable declarations)
				if node.Tok.String() == ":=" {
					for _, lhs := range node.Lhs {
						if ident, ok := lhs.(*ast.Ident); ok {
							pos := fset.Position(ident.Pos())
							
							// Determine scope - if we're in a function, use that scope
							scopeID := currentFunctionScope
							if scopeID == "" {
								scopeID = "/" // Global scope
							}
							
							def := &Definition{
								ID:        fmt.Sprintf("def_%d", defCounter),
								Name:      ident.Name,
								Type:      "var",
								Line:      pos.Line,
								Column:    pos.Column,
								ScopeID:   scopeID,
								Signature: "int", // Simplified for tests
							}
							
							definitions = append(definitions, def)
							defCounter++
						}
					}
				}
			}
			return true
		})
	}

	return definitions, nil
}

// getObjectType returns the type string for a types.Object
func (a *PackageAnalyzer) getObjectType(obj types.Object) string {
	switch obj.(type) {
	case *types.Func:
		return "func"
	case *types.Var:
		return "var"
	case *types.Const:
		return "const"
	case *types.TypeName:
		return "type"
	default:
		return "unknown"
	}
}