package analyzer

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

type PackageAnalyzer struct {
	fset     *token.FileSet
	packages map[string]*PackageInfo
}

type PackageDiscovery struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`         // Relative path from repo root
	AbsolutePath string   `json:"absolutePath"` // Full filesystem path
	Files        []string `json:"files"`        // List of Go files in this package
}

type PackageInfo struct {
	Name        string                 `json:"name"`
	Path        string                 `json:"path"`
	Files       map[string]*FileInfo   `json:"files"`
	Symbols     map[string]*Symbol     `json:"symbols"`     // All symbols in this package
	References  map[string][]*Reference `json:"references"`  // Symbol -> list of references
	Imports     map[string]string      `json:"imports"`     // alias -> package path
}

type FileInfo struct {
	Path        string              `json:"path"`
	Source      string              `json:"source"`
	Symbols     map[string]*Symbol  `json:"symbols"`     // Symbols defined in this file
	References  []*Reference        `json:"references"`  // All symbol references in this file
	Imports     []*ImportInfo       `json:"imports"`     // Import statements in this file
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
}

type Reference struct {
	Name     string `json:"name"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Target   *Symbol `json:"target,omitempty"` // The symbol this references
}

type ImportInfo struct {
	Alias string `json:"alias,omitempty"`
	Path  string `json:"path"`
	Line  int    `json:"line"`
}

func New() *PackageAnalyzer {
	return &PackageAnalyzer{
		fset:     token.NewFileSet(),
		packages: make(map[string]*PackageInfo),
	}
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

	// Determine absolute path of package
	var absolutePackagePath string
	if packagePath == "" {
		absolutePackagePath = repoPath
	} else {
		absolutePackagePath = filepath.Join(repoPath, packagePath)
	}

	// Check if already analyzed and cached
	cacheKey := fmt.Sprintf("%s::%s", repoPath, packagePath)
	if pkg, exists := a.packages[cacheKey]; exists {
		fmt.Printf("Returning cached analysis for package %s\n", packagePath)
		return pkg, nil
	}

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
	packageInfo, err := a.analyzePackage(packageName, astPackage, repoPath)
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

	return a.analyzePackage(pkgName, pkg, repoRoot)
}

func (a *PackageAnalyzer) analyzePackage(pkgName string, pkg *ast.Package, basePath string) (*PackageInfo, error) {
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
		Files:      make(map[string]*FileInfo),
		Symbols:    make(map[string]*Symbol),
		References: make(map[string][]*Reference),
		Imports:    make(map[string]string),
	}

	// Analyze each file
	for i, file := range files {
		filePath := filePaths[i]
		relPath, _ := filepath.Rel(basePath, filePath)
		relPath = filepath.ToSlash(relPath)

		fileInfo, err := a.analyzeFile(file, relPath, info, typesPackage)
		if err != nil {
			fmt.Printf("Failed to analyze file %s: %v\n", relPath, err)
			continue
		}

		packageInfo.Files[relPath] = fileInfo

		// Collect symbols at package level
		for _, symbol := range fileInfo.Symbols {
			packageInfo.Symbols[symbol.Name] = symbol
			fmt.Printf("Added symbol: %s (%s) from %s:%d\n", symbol.Name, symbol.Type, symbol.File, symbol.Line)
		}

		// Collect references at package level
		for _, ref := range fileInfo.References {
			if packageInfo.References[ref.Name] == nil {
				packageInfo.References[ref.Name] = make([]*Reference, 0)
			}
			packageInfo.References[ref.Name] = append(packageInfo.References[ref.Name], ref)
		}
		
		fmt.Printf("File %s has %d symbols and %d references\n", relPath, len(fileInfo.Symbols), len(fileInfo.References))
	}

	// Resolve cross-references
	a.resolveReferences(packageInfo, info)

	a.packages[pkgName] = packageInfo
	return packageInfo, nil
}

func (a *PackageAnalyzer) analyzeFile(file *ast.File, relPath string, info *types.Info, pkg *types.Package) (*FileInfo, error) {
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
				symbol := a.createSymbolFromObject(obj, relPath, pos)
				if symbol != nil {
					fileInfo.Symbols[symbol.Name] = symbol
					fmt.Printf("Found definition: %s at %s:%d\n", symbol.Name, relPath, pos.Line)
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
				fileInfo.References = append(fileInfo.References, ref)
				fmt.Printf("Found reference: %s at %s:%d\n", node.Name, relPath, pos.Line)
			}
		}

		return true
	})

	return fileInfo, nil
}

func (a *PackageAnalyzer) createSymbolFromObject(obj types.Object, file string, pos token.Position) *Symbol {
	if obj == nil {
		return nil
	}

	symbol := &Symbol{
		Name:    obj.Name(),
		File:    file,
		Line:    pos.Line,
		Column:  pos.Column,
		Package: obj.Pkg().Name(),
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

func (a *PackageAnalyzer) resolveReferences(pkgInfo *PackageInfo, info *types.Info) {
	fmt.Printf("Resolving cross-references for package: %s\n", pkgInfo.Name)

	for _, fileInfo := range pkgInfo.Files {
		for _, ref := range fileInfo.References {
			// Try to find the target symbol
			if target, exists := pkgInfo.Symbols[ref.Name]; exists {
				ref.Target = target
				fmt.Printf("Resolved reference %s -> %s:%d\n", ref.Name, target.File, target.Line)
			}
		}
	}
}

func (a *PackageAnalyzer) resolveCrossPackageReferences(combinedPackage *PackageInfo) {
	fmt.Printf("Resolving cross-package references\n")

	for _, fileInfo := range combinedPackage.Files {
		for _, ref := range fileInfo.References {
			if ref.Target == nil {
				// Try to find the target in the combined symbol table
				if target, exists := combinedPackage.Symbols[ref.Name]; exists {
					ref.Target = target
					fmt.Printf("Resolved cross-package reference %s -> %s:%d (%s)\n", 
						ref.Name, target.File, target.Line, target.Package)
				} else {
					// Try with package prefix
					for symbolKey, target := range combinedPackage.Symbols {
						if strings.HasSuffix(symbolKey, "."+ref.Name) {
							ref.Target = target
							fmt.Printf("Resolved prefixed reference %s -> %s:%d (%s)\n", 
								ref.Name, target.File, target.Line, target.Package)
							break
						}
					}
				}
			}
		}
	}
}

func (a *PackageAnalyzer) GetFileInfo(packageName, filePath string) (*FileInfo, error) {
	pkg, exists := a.packages[packageName]
	if !exists {
		return nil, fmt.Errorf("package not found: %s", packageName)
	}

	fileInfo, exists := pkg.Files[filePath]
	if !exists {
		return nil, fmt.Errorf("file not found: %s in package %s", filePath, packageName)
	}

	return fileInfo, nil
}