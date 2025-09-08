package analyzer

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"
)

// AnalysisQuality represents the completeness of package analysis
type AnalysisQuality struct {
	// IsComplete indicates if analysis had all dependencies available
	IsComplete bool `json:"is_complete"`
	
	// MissingDependencies lists dependencies that couldn't be resolved
	MissingDependencies []string `json:"missing_dependencies,omitempty"`
	
	// ImportErrors contains detailed information about import failures
	ImportErrors []ImportError `json:"import_errors,omitempty"`
	
	// AnalysisMode describes what level of analysis was possible
	AnalysisMode AnalysisMode `json:"analysis_mode"`
	
	// EnhancementAvailable indicates if re-analysis could provide better results
	EnhancementAvailable bool `json:"enhancement_available"`
	
	// QualityScore is a 0-1 score indicating analysis completeness
	QualityScore float64 `json:"quality_score"`
}

type ImportError struct {
	ImportPath string `json:"import_path"`
	Error      string `json:"error"`
	Position   string `json:"position,omitempty"`
	Severity   string `json:"severity"` // "error", "warning"
}

type AnalysisMode string

const (
	// AnalysisModeComplete indicates full analysis with all dependencies
	AnalysisModeComplete AnalysisMode = "complete"
	
	// AnalysisModePartial indicates some dependencies missing but core analysis works
	AnalysisModePartial AnalysisMode = "partial"
	
	// AnalysisModeSyntaxOnly indicates only syntax-level analysis possible
	AnalysisModeSyntaxOnly AnalysisMode = "syntax_only"
	
	// AnalysisModeFailed indicates analysis failed completely
	AnalysisModeFailed AnalysisMode = "failed"
)

// AssessAnalysisQuality evaluates the quality and completeness of package analysis
func AssessAnalysisQuality(pkg *packages.Package) *AnalysisQuality {
	quality := &AnalysisQuality{
		MissingDependencies: make([]string, 0),
		ImportErrors:        make([]ImportError, 0),
	}
	
	// Start with optimistic assumptions
	quality.IsComplete = true
	quality.AnalysisMode = AnalysisModeComplete
	quality.QualityScore = 1.0
	
	// Analyze package errors to determine completeness
	importErrors := 0
	totalImports := len(pkg.Imports)
	
	for _, pkgErr := range pkg.Errors {
		importErr := ImportError{
			Error:    pkgErr.Error(),
			Position: pkgErr.Pos,
			Severity: "error",
		}
		
		// Parse import path from error message
		if strings.Contains(pkgErr.Error(), "could not import") {
			importErr.ImportPath = extractImportPathFromError(pkgErr.Error())
			quality.MissingDependencies = append(quality.MissingDependencies, importErr.ImportPath)
			importErrors++
			quality.IsComplete = false
		}
		
		quality.ImportErrors = append(quality.ImportErrors, importErr)
	}
	
	// Check import status for additional missing dependencies
	for importPath, importedPkg := range pkg.Imports {
		if importedPkg != nil && len(importedPkg.Errors) > 0 {
			// This import has errors, but may not have been counted above
			hasImportError := false
			for _, existing := range quality.MissingDependencies {
				if existing == importPath {
					hasImportError = true
					break
				}
			}
			if !hasImportError {
				quality.MissingDependencies = append(quality.MissingDependencies, importPath)
				importErrors++
				quality.IsComplete = false
			}
		}
	}
	
	// Determine analysis mode based on what succeeded
	if pkg.Types == nil || pkg.TypesInfo == nil {
		quality.AnalysisMode = AnalysisModeFailed
		quality.QualityScore = 0.0
	} else if len(quality.MissingDependencies) == 0 {
		quality.AnalysisMode = AnalysisModeComplete
		quality.QualityScore = 1.0
	} else if pkg.Types != nil && pkg.TypesInfo != nil {
		quality.AnalysisMode = AnalysisModePartial
		// Calculate quality score based on successful imports
		if totalImports > 0 {
			successfulImports := totalImports - importErrors
			quality.QualityScore = float64(successfulImports) / float64(totalImports)
		} else {
			quality.QualityScore = 0.8 // Reasonable score for packages without imports
		}
	} else {
		quality.AnalysisMode = AnalysisModeSyntaxOnly
		quality.QualityScore = 0.3
	}
	
	// Enhancement is available if we have missing dependencies that could potentially be resolved
	quality.EnhancementAvailable = len(quality.MissingDependencies) > 0 && 
		quality.AnalysisMode != AnalysisModeFailed
	
	return quality
}

// extractImportPathFromError parses import path from packages error message
func extractImportPathFromError(errorMsg string) string {
	// Error format: "could not import github.com/gin-gonic/gin (invalid package name: \"\")"
	if strings.Contains(errorMsg, "could not import ") {
		start := strings.Index(errorMsg, "could not import ") + len("could not import ")
		end := strings.Index(errorMsg[start:], " ")
		if end == -1 {
			end = len(errorMsg) - start
		}
		return errorMsg[start : start+end]
	}
	return ""
}

// DependencyLoadingStatus represents the status of dependency loading operation
type DependencyLoadingStatus struct {
	// Status indicates current state
	Status LoadingStatus `json:"status"`
	
	// Progress indicates how many dependencies have been processed
	Progress DependencyProgress `json:"progress"`
	
	// EstimatedCompletion gives rough time estimate if available
	EstimatedCompletion string `json:"estimated_completion,omitempty"`
	
	// LoadedDependencies lists dependencies that have been successfully loaded
	LoadedDependencies []string `json:"loaded_dependencies,omitempty"`
	
	// FailedDependencies lists dependencies that failed to load
	FailedDependencies []string `json:"failed_dependencies,omitempty"`
}

type LoadingStatus string

const (
	LoadingStatusIdle       LoadingStatus = "idle"
	LoadingStatusInProgress LoadingStatus = "in_progress" 
	LoadingStatusComplete   LoadingStatus = "complete"
	LoadingStatusFailed     LoadingStatus = "failed"
)

type DependencyProgress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// EnhancedAnalysisResponse contains both analysis results and quality information
type EnhancedAnalysisResponse struct {
	// Standard analysis results
	*PackageInfo `json:"package_info,omitempty"`
	*FileInfo    `json:"file_info,omitempty"`
	
	// Quality assessment
	Quality *AnalysisQuality `json:"quality"`
	
	// Dependency loading status
	DependencyStatus *DependencyLoadingStatus `json:"dependency_status,omitempty"`
	
	// Enhancement token for requesting enhanced analysis
	EnhancementToken string `json:"enhancement_token,omitempty"`
}

// GenerateEnhancementToken creates a token that can be used to request enhanced analysis
func GenerateEnhancementToken(packagePath string, missingDeps []string) string {
	// In a real implementation, this might be a JWT or database key
	// For now, simple string encoding
	return fmt.Sprintf("enhance_%s_%d", packagePath, len(missingDeps))
}