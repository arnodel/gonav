package analyzer

import (
	"fmt"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

// AnalyzePackageWithQuality performs package analysis and returns enhanced results with quality assessment
func (pa *PackagesAnalyzer) AnalyzePackageWithQuality(packagePath string) (*EnhancedAnalysisResponse, error) {
	// Load the specific package
	pattern := "./" + packagePath
	if packagePath == "" {
		pattern = "./..."
	}

	pkgs, err := packages.Load(pa.config, pattern)
	if err != nil {
		return &EnhancedAnalysisResponse{
			Quality: &AnalysisQuality{
				IsComplete:           false,
				AnalysisMode:         AnalysisModeFailed,
				EnhancementAvailable: false,
				QualityScore:         0.0,
				ImportErrors: []ImportError{
					{Error: err.Error(), Severity: "error"},
				},
			},
		}, fmt.Errorf("failed to load package %s: %w", packagePath, err)
	}

	if len(pkgs) == 0 {
		return &EnhancedAnalysisResponse{
			Quality: &AnalysisQuality{
				IsComplete:           false,
				AnalysisMode:         AnalysisModeFailed,
				EnhancementAvailable: false,
				QualityScore:         0.0,
			},
		}, fmt.Errorf("no packages found for pattern %s", pattern)
	}

	pkg := pkgs[0]
	
	// Assess analysis quality
	quality := AssessAnalysisQuality(pkg)
	
	// Log quality information
	fmt.Printf("Analysis quality for %s: mode=%s, score=%.2f, missing_deps=%d\n", 
		pkg.PkgPath, quality.AnalysisMode, quality.QualityScore, len(quality.MissingDependencies))
	
	// Convert to package info
	packageInfo, err := pa.convertPackageToPackageInfo(pkg)
	if err != nil {
		return &EnhancedAnalysisResponse{
			Quality: quality,
		}, err
	}

	response := &EnhancedAnalysisResponse{
		PackageInfo: packageInfo,
		Quality:     quality,
	}

	// Generate enhancement token if enhancement is available
	if quality.EnhancementAvailable {
		response.EnhancementToken = GenerateEnhancementToken(packagePath, quality.MissingDependencies)
	}

	return response, nil
}

// AnalyzeSingleFileWithQuality performs file analysis and returns enhanced results with quality assessment
func (pa *PackagesAnalyzer) AnalyzeSingleFileWithQuality(filePath string) (*EnhancedAnalysisResponse, error) {
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
		return &EnhancedAnalysisResponse{
			Quality: &AnalysisQuality{
				IsComplete:           false,
				AnalysisMode:         AnalysisModeFailed,
				EnhancementAvailable: false,
				QualityScore:         0.0,
				ImportErrors: []ImportError{
					{Error: err.Error(), Severity: "error"},
				},
			},
		}, fmt.Errorf("failed to load package for file %s: %w", filePath, err)
	}

	// Find the package containing our file
	var targetPkg *packages.Package
	for _, pkg := range pkgs {
		for _, file := range pkg.CompiledGoFiles {
			if filepath.Base(file) == filepath.Base(filePath) {
				targetPkg = pkg
				break
			}
		}
		if targetPkg != nil {
			break
		}
	}

	if targetPkg == nil {
		return &EnhancedAnalysisResponse{
			Quality: &AnalysisQuality{
				IsComplete:           false,
				AnalysisMode:         AnalysisModeFailed,
				EnhancementAvailable: false,
				QualityScore:         0.0,
			},
		}, fmt.Errorf("could not find package containing file %s", filePath)
	}

	// Assess analysis quality
	quality := AssessAnalysisQuality(targetPkg)
	
	// Log quality information
	fmt.Printf("File analysis quality for %s: mode=%s, score=%.2f, missing_deps=%d\n", 
		filePath, quality.AnalysisMode, quality.QualityScore, len(quality.MissingDependencies))
	
	// Convert to file info
	fileInfo, err := pa.convertPackageToFileInfo(targetPkg, filePath)
	if err != nil {
		return &EnhancedAnalysisResponse{
			Quality: quality,
		}, err
	}

	response := &EnhancedAnalysisResponse{
		FileInfo: fileInfo,
		Quality:  quality,
	}

	// Generate enhancement token if enhancement is available
	if quality.EnhancementAvailable {
		response.EnhancementToken = GenerateEnhancementToken(filePath, quality.MissingDependencies)
	}

	return response, nil
}

// GetDependencyLoadingStatus returns current status of dependency loading for a package
func (pa *PackagesAnalyzer) GetDependencyLoadingStatus(enhancementToken string) (*DependencyLoadingStatus, error) {
	// TODO: Implement actual dependency loading status tracking
	// This would typically involve:
	// 1. Parse enhancement token to identify package/dependencies
	// 2. Check status of background dependency loading
	// 3. Return current progress
	
	return &DependencyLoadingStatus{
		Status: LoadingStatusIdle,
		Progress: DependencyProgress{
			Total:     0,
			Completed: 0,
			Failed:    0,
		},
	}, nil
}

// TriggerDependencyLoading initiates background loading of missing dependencies
func (pa *PackagesAnalyzer) TriggerDependencyLoading(enhancementToken string) error {
	// TODO: Implement actual dependency loading
	// This would typically involve:
	// 1. Parse enhancement token to identify missing dependencies  
	// 2. Spawn background goroutine to run `go mod download`
	// 3. Update loading status as dependencies are resolved
	// 4. Optionally notify when enhancement is ready
	
	fmt.Printf("Dependency loading triggered for token: %s\n", enhancementToken)
	return nil
}