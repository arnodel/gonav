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

func (a *PackageAnalyzer) AnalyzeRepository(repoPath string) (*PackageInfo, error) {
	fmt.Printf("Analyzing repository at: %s\n", repoPath)

	// Find all packages in the repository (including subdirectories)
	allPackages, err := a.findAllPackages(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find packages: %w", err)
	}

	fmt.Printf("Found %d packages total:\n", len(allPackages))
	for pkgPath, pkgName := range allPackages {
		fmt.Printf("  - Package '%s' at %s\n", pkgName, pkgPath)
	}

	// Analyze all packages and merge them into one comprehensive view
	// For now, we'll create a "virtual" package that combines all symbols
	combinedPackage := &PackageInfo{
		Name:       "combined",
		Path:       repoPath,
		Files:      make(map[string]*FileInfo),
		Symbols:    make(map[string]*Symbol),
		References: make(map[string][]*Reference),
		Imports:    make(map[string]string),
	}

	for pkgPath, pkgName := range allPackages {
		fmt.Printf("Analyzing package '%s' at %s\n", pkgName, pkgPath)
		
		pkgInfo, err := a.analyzeSinglePackage(pkgName, pkgPath, repoPath)
		if err != nil {
			fmt.Printf("Failed to analyze package %s: %v\n", pkgName, err)
			continue
		}

		// Merge package into combined view
		for filePath, fileInfo := range pkgInfo.Files {
			combinedPackage.Files[filePath] = fileInfo
		}

		for symbolName, symbol := range pkgInfo.Symbols {
			// Prefix symbol name with package if there are conflicts
			key := symbolName
			if _, exists := combinedPackage.Symbols[key]; exists && symbol.Package != combinedPackage.Symbols[key].Package {
				key = fmt.Sprintf("%s.%s", symbol.Package, symbolName)
			}
			combinedPackage.Symbols[key] = symbol
		}

		for refName, refs := range pkgInfo.References {
			if combinedPackage.References[refName] == nil {
				combinedPackage.References[refName] = make([]*Reference, 0)
			}
			combinedPackage.References[refName] = append(combinedPackage.References[refName], refs...)
		}
	}

	// Resolve cross-package references
	a.resolveCrossPackageReferences(combinedPackage)

	fmt.Printf("Combined package has %d symbols across %d files\n", len(combinedPackage.Symbols), len(combinedPackage.Files))
	return combinedPackage, nil
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