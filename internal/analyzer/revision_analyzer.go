package analyzer

import (
	"fmt"
	"time"
)

// RevisionAnalyzer combines packages analysis with revision-based caching and progressive enhancement
type RevisionAnalyzer struct {
	packagesAnalyzer *PackagesAnalyzer
	cache            *AnalysisCache
	dependencyQueue  *DependencyQueue
	
	// Configuration
	repoPath string
	env      []string
}

// RevisionAnalysisResponse represents a response with revision tracking
type RevisionAnalysisResponse struct {
	// Analysis results (one of these will be set)
	PackageInfo *PackageInfo `json:"package_info,omitempty"`
	FileInfo    *FileInfo    `json:"file_info,omitempty"`
	
	// Revision tracking
	Revision string `json:"revision"`
	Complete bool   `json:"complete"`
	NoChange bool   `json:"no_change,omitempty"`
	
	// Optional quality information (for debugging/monitoring)
	Quality *AnalysisQuality `json:"quality,omitempty"`
}

// NewRevisionAnalyzer creates a new revision-based analyzer
func NewRevisionAnalyzer(repoPath string, env []string, queueConfig DependencyQueueConfig) *RevisionAnalyzer {
	dependencyChecker := &SimpleDependencyChecker{}
	
	return &RevisionAnalyzer{
		packagesAnalyzer: NewPackagesAnalyzer(repoPath, env),
		cache:           NewAnalysisCache(dependencyChecker),
		dependencyQueue: NewDependencyQueue(queueConfig),
		repoPath:        repoPath,
		env:             env,
	}
}

// AnalyzePackage performs revision-based package analysis
func (ra *RevisionAnalyzer) AnalyzePackage(packagePath, clientRevision string) (*RevisionAnalysisResponse, error) {
	key := CacheKey{
		Type:        CacheKeyTypePackage,
		PackagePath: packagePath,
	}
	
	return ra.analyzeWithCache(key, clientRevision, func() (*CachedAnalysis, error) {
		return ra.performPackageAnalysis(packagePath)
	})
}

// AnalyzeFile performs revision-based file analysis
func (ra *RevisionAnalyzer) AnalyzeFile(packagePath, filePath, clientRevision string) (*RevisionAnalysisResponse, error) {
	key := CacheKey{
		Type:        CacheKeyTypeFile,
		PackagePath: packagePath,
		FilePath:    filePath,
	}
	
	return ra.analyzeWithCache(key, clientRevision, func() (*CachedAnalysis, error) {
		return ra.performFileAnalysis(filePath)
	})
}

// analyzeWithCache implements the core revision-based analysis logic
func (ra *RevisionAnalyzer) analyzeWithCache(key CacheKey, clientRevision string, analyzer func() (*CachedAnalysis, error)) (*RevisionAnalysisResponse, error) {
	// Step 1: Check cache
	cached, cacheResult := ra.cache.Get(key, clientRevision)
	
	switch cacheResult {
	case CacheResultNoChange:
		// Client has same revision, return no change
		return &RevisionAnalysisResponse{
			Revision: cached.Revision,
			Complete: cached.IsComplete,
			NoChange: true,
		}, nil
		
	case CacheResultNewer:
		// Cache has newer revision, return it
		return ra.buildResponse(cached), nil
		
	case CacheResultHit:
		// First request or returning cached version
		// Check if we should trigger dependency loading
		if !cached.IsComplete && !ra.dependencyQueue.IsActive(key) {
			ra.triggerDependencyLoading(key, cached)
		}
		return ra.buildResponse(cached), nil
		
	case CacheResultMiss:
		// No cache entry, need to analyze
		// Fall through to analysis
	}
	
	// Step 2: Check if we should recalculate (for cache miss or potential improvement)
	shouldRecalc, availableDeps, err := ra.cache.ShouldRecalculate(key, ra.repoPath)
	if err != nil {
		return nil, fmt.Errorf("error checking recalculation need: %w", err)
	}
	
	if cached != nil && !shouldRecalc {
		// Cache exists but no recalculation needed, return cached
		return ra.buildResponse(cached), nil
	}
	
	if shouldRecalc && len(availableDeps) > 0 {
		fmt.Printf("Recalculating analysis for %s: %d new dependencies available\n", key.String(), len(availableDeps))
	}
	
	// Step 3: Perform analysis
	newAnalysis, err := analyzer()
	if err != nil {
		return nil, err
	}
	
	// Step 4: Cache the new analysis
	ra.cache.Set(key, newAnalysis)
	
	// Step 5: Trigger dependency loading if incomplete
	if !newAnalysis.IsComplete && !ra.dependencyQueue.IsActive(key) {
		ra.triggerDependencyLoading(key, newAnalysis)
	}
	
	return ra.buildResponse(newAnalysis), nil
}

// performPackageAnalysis performs actual package analysis
func (ra *RevisionAnalyzer) performPackageAnalysis(packagePath string) (*CachedAnalysis, error) {
	// Use enhanced analysis to get quality information
	enhancedResponse, err := ra.packagesAnalyzer.AnalyzePackageWithQuality(packagePath)
	if err != nil {
		return nil, err
	}
	
	// Calculate revision based on analysis state
	symbolCount := len(enhancedResponse.PackageInfo.Symbols)
	refCount := 0 // Package analysis doesn't have references
	revision := GenerateRevision(packagePath, enhancedResponse.Quality, symbolCount, refCount)
	
	return &CachedAnalysis{
		Revision:                revision,
		PackageInfo:             enhancedResponse.PackageInfo,
		Quality:                 enhancedResponse.Quality,
		Timestamp:               time.Now(),
		MissingDependencies:     enhancedResponse.Quality.MissingDependencies,
		DependencyLoadingInProgress: false,
		IsComplete:              enhancedResponse.Quality.IsComplete,
	}, nil
}

// performFileAnalysis performs actual file analysis
func (ra *RevisionAnalyzer) performFileAnalysis(filePath string) (*CachedAnalysis, error) {
	// Use enhanced analysis to get quality information
	enhancedResponse, err := ra.packagesAnalyzer.AnalyzeSingleFileWithQuality(filePath)
	if err != nil {
		return nil, err
	}
	
	// Calculate revision based on analysis state
	symbolCount := len(enhancedResponse.FileInfo.Symbols)
	refCount := len(enhancedResponse.FileInfo.References)
	revision := GenerateRevision(filePath, enhancedResponse.Quality, symbolCount, refCount)
	
	return &CachedAnalysis{
		Revision:                revision,
		FileInfo:                enhancedResponse.FileInfo,
		Quality:                 enhancedResponse.Quality,
		Timestamp:               time.Now(),
		MissingDependencies:     enhancedResponse.Quality.MissingDependencies,
		DependencyLoadingInProgress: false,
		IsComplete:              enhancedResponse.Quality.IsComplete,
	}, nil
}

// triggerDependencyLoading starts background dependency loading
func (ra *RevisionAnalyzer) triggerDependencyLoading(key CacheKey, cached *CachedAnalysis) {
	if len(cached.MissingDependencies) == 0 {
		return
	}
	
	// Mark dependency loading as in progress
	ra.cache.MarkDependencyLoadingInProgress(key, true)
	
	// Create download request
	req := DependencyDownloadRequest{
		WorkDir:      ra.repoPath,
		Dependencies: cached.MissingDependencies,
		CacheKey:     key,
		RequestID:    fmt.Sprintf("%s_%d", key.String(), time.Now().Unix()),
		ResultChan:   make(chan DependencyDownloadResult, 1),
	}
	
	// Submit to queue
	err := ra.dependencyQueue.SubmitDownloadRequest(req)
	if err != nil {
		fmt.Printf("Failed to submit dependency download request: %v\n", err)
		ra.cache.MarkDependencyLoadingInProgress(key, false)
		return
	}
	
	// Start goroutine to handle completion
	go ra.handleDependencyLoadingResult(key, req.ResultChan)
	
	fmt.Printf("Triggered dependency loading for %s: %v\n", key.String(), cached.MissingDependencies)
}

// handleDependencyLoadingResult handles the completion of dependency loading
func (ra *RevisionAnalyzer) handleDependencyLoadingResult(key CacheKey, resultChan chan DependencyDownloadResult) {
	select {
	case result := <-resultChan:
		// Mark loading as complete
		ra.cache.MarkDependencyLoadingInProgress(key, false)
		
		fmt.Printf("Dependency loading completed for %s: success=%d, failed=%d\n", 
			key.String(), len(result.Successful), len(result.Failed))
		
		// If any dependencies were successfully loaded, the next analysis request will recalculate
		// No need to pro-actively recalculate here
		
	case <-time.After(10 * time.Minute): // Timeout
		ra.cache.MarkDependencyLoadingInProgress(key, false)
		fmt.Printf("Dependency loading timed out for %s\n", key.String())
	}
}

// buildResponse creates a RevisionAnalysisResponse from cached analysis
func (ra *RevisionAnalyzer) buildResponse(cached *CachedAnalysis) *RevisionAnalysisResponse {
	response := &RevisionAnalysisResponse{
		Revision: cached.Revision,
		Complete: cached.IsComplete,
		Quality:  cached.Quality, // Optional, for debugging
	}
	
	if cached.PackageInfo != nil {
		response.PackageInfo = cached.PackageInfo
	}
	
	if cached.FileInfo != nil {
		response.FileInfo = cached.FileInfo
	}
	
	return response
}

// GetCacheStats returns cache statistics
func (ra *RevisionAnalyzer) GetCacheStats() CacheStats {
	return ra.cache.GetStats()
}

// GetQueueStats returns dependency queue statistics
func (ra *RevisionAnalyzer) GetQueueStats() DependencyQueueStats {
	return ra.dependencyQueue.GetStats()
}

// Cleanup performs maintenance on cache and queue
func (ra *RevisionAnalyzer) Cleanup(maxAge time.Duration) {
	removed := ra.cache.Cleanup(maxAge)
	if removed > 0 {
		fmt.Printf("Cleaned up %d old cache entries\n", removed)
	}
}

// Shutdown gracefully shuts down the revision analyzer
func (ra *RevisionAnalyzer) Shutdown(timeout time.Duration) error {
	fmt.Println("Shutting down revision analyzer...")
	return ra.dependencyQueue.Shutdown(timeout)
}