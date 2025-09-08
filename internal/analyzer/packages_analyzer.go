package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// PackagesAnalyzer uses golang.org/x/tools/go/packages for robust package analysis
type PackagesAnalyzer struct {
	config     *packages.Config
	moduleInfo *ModuleInfo // Module context for resolving external references
}

// NewPackagesAnalyzer creates a new packages-based analyzer
func NewPackagesAnalyzer(repoPath string, env []string) *PackagesAnalyzer {
	return &PackagesAnalyzer{
		config: &packages.Config{
			Mode: packages.NeedName |
				packages.NeedFiles |
				packages.NeedCompiledGoFiles |
				packages.NeedImports |
				packages.NeedTypes |
				packages.NeedSyntax |
				packages.NeedTypesInfo |
				packages.NeedTypesSizes,
			Dir:   repoPath,
			Env:   env,
			Tests: false, // We'll handle test files separately if needed
		},
		moduleInfo: nil, // Will be set when analyzing
	}
}

// SetModuleContext sets the module context for resolving external references
func (pa *PackagesAnalyzer) SetModuleContext(moduleInfo *ModuleInfo) {
	pa.moduleInfo = moduleInfo
}

// AnalyzePackageWithPackages analyzes a package using golang.org/x/tools/go/packages
func (pa *PackagesAnalyzer) AnalyzePackageWithPackages(packagePath string) (*PackageInfo, error) {
	// Load the specific package
	pattern := "./" + packagePath
	if packagePath == "" {
		pattern = "./..."
	}

	pkgs, err := packages.Load(pa.config, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to load package %s: %w", packagePath, err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found for pattern %s", pattern)
	}

	// For now, analyze the first package found
	pkg := pkgs[0]
	
	// Check for errors in package loading
	if len(pkg.Errors) > 0 {
		// Log errors but continue with partial analysis
		for _, err := range pkg.Errors {
			fmt.Printf("Package loading warning: %v\n", err)
		}
	}

	return pa.convertPackageToPackageInfo(pkg)
}

// AnalyzeSingleFileWithPackages analyzes a single file using packages
func (pa *PackagesAnalyzer) AnalyzeSingleFileWithPackages(filePath string) (*FileInfo, error) {
	// First, determine which package this file belongs to
	dir := filepath.Dir(filePath)
	relativeDir, err := filepath.Rel(pa.config.Dir, dir)
	if err != nil {
		relativeDir = "."
	}
	
	pattern := "./" + relativeDir
	if relativeDir == "." {
		pattern = "./..."
	}

	pkgs, err := packages.Load(pa.config, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to load package for file %s: %w", filePath, err)
	}

	// Find the package containing our file
	var targetPkg *packages.Package
	for _, pkg := range pkgs {
		for _, file := range pkg.CompiledGoFiles {
			if strings.HasSuffix(file, filePath) {
				targetPkg = pkg
				break
			}
		}
		if targetPkg != nil {
			break
		}
	}

	if targetPkg == nil {
		return nil, fmt.Errorf("could not find package containing file %s", filePath)
	}

	return pa.convertPackageToFileInfo(targetPkg, filePath)
}

// convertPackageToPackageInfo converts a packages.Package to our PackageInfo format
func (pa *PackagesAnalyzer) convertPackageToPackageInfo(pkg *packages.Package) (*PackageInfo, error) {
	packageInfo := &PackageInfo{
		Name:    pkg.Name,
		Path:    pkg.PkgPath,
		Files:   make([]FileEntry, 0),
		Symbols: make(map[string]*Symbol),
	}

	// Add files
	for _, file := range pkg.CompiledGoFiles {
		rel, err := filepath.Rel(pa.config.Dir, file)
		if err != nil {
			rel = file
		}
		packageInfo.Files = append(packageInfo.Files, FileEntry{
			Path: filepath.ToSlash(rel),
			IsGo: true,
		})
	}

	// Extract symbols using type information
	if pkg.Types != nil && pkg.TypesInfo != nil {
		symbols := pa.extractSymbolsFromPackage(pkg)
		for _, symbol := range symbols {
			packageInfo.Symbols[symbol.Name] = &symbol
		}
	}

	return packageInfo, nil
}

// convertPackageToFileInfo converts a packages.Package to FileInfo for a specific file
func (pa *PackagesAnalyzer) convertPackageToFileInfo(pkg *packages.Package, targetFilePath string) (*FileInfo, error) {
	// Find the AST node for the target file
	var targetFile *ast.File
	var targetFileContent string
	
	for i, file := range pkg.CompiledGoFiles {
		if strings.HasSuffix(file, targetFilePath) {
			if i < len(pkg.Syntax) {
				targetFile = pkg.Syntax[i]
			}
			// Read file content
			content, err := readFileContent(file)
			if err != nil {
				return nil, fmt.Errorf("failed to read file content: %w", err)
			}
			targetFileContent = content
			break
		}
	}

	if targetFile == nil {
		return nil, fmt.Errorf("could not find AST for file %s", targetFilePath)
	}

	fileInfo := &FileInfo{
		Source:      targetFileContent,
		References:  make([]*Reference, 0),
		Symbols:     make(map[string]*Symbol),
		Scopes:      make([]*ScopeInfo, 0),
		Definitions: make([]*Definition, 0),
	}

	// Extract symbols and references for this specific file
	if pkg.TypesInfo != nil {
		pa.extractFileSymbolsAndReferences(targetFile, pkg, fileInfo)
	}

	return fileInfo, nil
}

// extractSymbolsFromPackage extracts symbols from a package using type information
func (pa *PackagesAnalyzer) extractSymbolsFromPackage(pkg *packages.Package) []Symbol {
	var symbols []Symbol

	// Extract from package scope (both exported and unexported)
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj == nil {
			continue
		}

		symbol := pa.convertObjectToSymbol(obj, pkg)
		if symbol != nil {
			symbols = append(symbols, *symbol)
		}
		
		// Note: We intentionally do NOT extract methods here as they would cause
		// key collisions in the symbols map (methods vs functions with same name)
	}

	return symbols
}

// extractFileSymbolsAndReferences extracts symbols and references for a specific file
func (pa *PackagesAnalyzer) extractFileSymbolsAndReferences(file *ast.File, pkg *packages.Package, fileInfo *FileInfo) {
	fset := pkg.Fset

	// Walk the AST to find identifiers and their usage
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.Ident:
			pos := fset.Position(node.Pos())
			
			// Check if this identifier has type information
			if obj, ok := pkg.TypesInfo.Uses[node]; ok {
				// This is a use of an identifier
				ref := &Reference{
					Name:   node.Name,
					File:   "", // Will be filled in by caller
					Line:   pos.Line,
					Column: pos.Column,
				}
				
				// Try to create target symbol
				if targetSymbol := pa.convertObjectToSymbol(obj, pkg); targetSymbol != nil {
					ref.Target = targetSymbol
				}
				
				fileInfo.References = append(fileInfo.References, ref)
			}
			
			if obj, ok := pkg.TypesInfo.Defs[node]; ok && obj != nil {
				// This is a definition of an identifier
				symbol := pa.convertObjectToSymbol(obj, pkg)
				if symbol != nil {
					fileInfo.Symbols[symbol.Name] = symbol
				}
				
				def := &Definition{
					ID:     fmt.Sprintf("def_%s_%d", node.Name, pos.Line),
					Name:   node.Name,
					Type:   pa.getObjectKind(obj),
					Line:   pos.Line,
					Column: pos.Column,
					ScopeID: "/", // Simplified for now
					Signature: obj.String(),
				}
				fileInfo.Definitions = append(fileInfo.Definitions, def)
			}
		}
		return true
	})
}

// convertObjectToSymbol converts a types.Object to our Symbol format
func (pa *PackagesAnalyzer) convertObjectToSymbol(obj types.Object, pkg *packages.Package) *Symbol {
	if obj == nil {
		return nil
	}

	pos := pkg.Fset.Position(obj.Pos())
	
	// Handle file path - packages provides position info for external symbols too
	file := ""
	if pos.IsValid() && pos.Filename != "" {
		if obj.Pkg() != nil && obj.Pkg().Path() == pkg.PkgPath {
			// For current package symbols, use relative path
			if relPath, err := filepath.Rel(pa.config.Dir, pos.Filename); err == nil {
				file = filepath.ToSlash(relPath)
			}
		} else {
			// For external symbols, we need to distinguish between same-repo and cross-repo
			filename := pos.Filename
			
			// Check if this is from the same repository by checking if the path is within pa.config.Dir
			if relPath, err := filepath.Rel(pa.config.Dir, filename); err == nil && !strings.HasPrefix(relPath, "..") {
				// Same repository, different package - use relative path
				file = filepath.ToSlash(relPath)
			} else {
				// Different repository - extract relative path within target repository
				file = pa.extractRelativeFilePathFromCache(filename)
			}
		}
	}
	
	// Handle package name and path
	packageName := ""
	importPath := ""
	if obj.Pkg() != nil {
		packageName = obj.Pkg().Name()
		importPath = obj.Pkg().Path()
	} else {
		packageName = "builtin"
	}
	
	// For external references, we need to convert cache paths to module@version format
	isExternal := obj.Pkg() != nil && obj.Pkg().Path() != pkg.PkgPath
	isStdLib := pa.isStandardLibraryImport(importPath)
	
	symbol := &Symbol{
		Name:       obj.Name(),
		Type:       pa.getObjectKind(obj),
		File:       file,
		Line:       pos.Line,
		Column:     pos.Column,
		Package:    packageName,
		Signature:  obj.String(),
		ImportPath: importPath,
		IsExternal: isExternal,
		IsStdLib:   isStdLib,
	}
	
	// For external references, resolve module@version format
	if isExternal && pa.moduleInfo != nil && !isStdLib {
		resolvedPath, version := pa.moduleInfo.ResolveImport(importPath)
		symbol.ImportPath = resolvedPath
		symbol.Version = version
		
		// Use the resolved import path for the Package field for cross-module navigation
		if version != "" {
			symbol.Package = resolvedPath + "@" + version
		} else {
			symbol.Package = resolvedPath
		}
	}
	
	return symbol
}

// getObjectKind returns the kind of a types.Object
func (pa *PackagesAnalyzer) getObjectKind(obj types.Object) string {
	switch obj.(type) {
	case *types.Func:
		return "function"
	case *types.Var:
		return "variable"
	case *types.Const:
		return "constant"
	case *types.TypeName:
		return "type"
	case *types.PkgName:
		return "package"
	default:
		return "unknown"
	}
}

// getObjectTarget returns the target location of a types.Object
func (pa *PackagesAnalyzer) getObjectTarget(obj types.Object) string {
	if obj.Pkg() == nil {
		return ""
	}
	return obj.Pkg().Path()
}

// isExternal checks if an object is from an external package
func (pa *PackagesAnalyzer) isExternal(obj types.Object, pkg *packages.Package) bool {
	if obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path() != pkg.PkgPath
}

// isStandardLibraryImport determines if an import path is from the Go standard library
// using module context for better accuracy
func (pa *PackagesAnalyzer) isStandardLibraryImport(importPath string) bool {
	if importPath == "" {
		return false
	}
	
	// Local/main packages are not standard library
	if importPath == "main" {
		return false
	}
	
	// Builtin is a special pseudo-package, not standard library
	if importPath == "builtin" {
		return false
	}
	
	// If we have module context, check if this is a subpackage of the current module
	if pa.moduleInfo != nil {
		// If the import path starts with the current module path, it's not stdlib
		if strings.HasPrefix(importPath, pa.moduleInfo.ModulePath+"/") || importPath == pa.moduleInfo.ModulePath {
			return false
		}
	}
	
	// Standard library packages don't contain dots (domain names)
	// This is a reliable way to detect them since all external packages
	// should have domain names like github.com/user/repo
	return !strings.Contains(importPath, ".")
}

// readFileContent reads the content of a file
func readFileContent(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return string(content), nil
}

// extractRelativeFilePathFromCache extracts the relative file path within 
// a repository from cache paths like:
// .../gomodcache/github.com/module@version/subdir/file.go -> subdir/file.go
// .../gonav-cache/github.com_module_version/file.go -> file.go
// Returns empty string for standard library paths (not in cache)
func (pa *PackagesAnalyzer) extractRelativeFilePathFromCache(filename string) string {
	// Two patterns to handle:
	// 1. gonav-cache/isolated-env/gomodcache/github.com/module@version/file.go -> file.go
	// 2. gonav-cache/github.com_module_version/file.go -> file.go  
	if strings.Contains(filename, "gomodcache") && strings.Contains(filename, "@") {
		// Pattern: .../gomodcache/github.com/module@version/subdir/file.go
		parts := strings.Split(filename, "gomodcache/")
		if len(parts) >= 2 {
			// parts[1] would be like "github.com/module@version/subdir/file.go"
			modCachePart := parts[1]
			// Find the first slash after @version
			atIndex := strings.Index(modCachePart, "@")
			if atIndex > 0 {
				// Find the next slash after the version
				nextSlash := strings.Index(modCachePart[atIndex:], "/")
				if nextSlash > 0 {
					// Extract everything after the version slash
					return filepath.ToSlash(modCachePart[atIndex+nextSlash+1:])
				}
			}
		}
	} else if strings.Contains(filename, "gonav-cache") {
		// Fallback: Handle our custom cache format
		parts := strings.Split(filename, "gonav-cache")
		if len(parts) >= 2 {
			cachePart := parts[1]
			if len(cachePart) > 1 && cachePart[0] == '/' {
				cachePart = cachePart[1:]
			}
			slashIndex := strings.Index(cachePart, "/")
			if slashIndex > 0 && slashIndex < len(cachePart)-1 {
				return filepath.ToSlash(cachePart[slashIndex+1:])
			}
		}
	}
	
	// Fallback: if not from cache (e.g. standard library), return empty string
	// This preserves the original behavior where external stdlib refs have empty files
	// Exception: empty string should return "." (filepath.Base behavior)
	if filename == "" {
		return "."
	}
	return ""
}