package analyzer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CacheKey represents the key for caching analysis results
type CacheKey struct {
	Type        CacheKeyType `json:"type"`        // "package" or "file"
	PackagePath string       `json:"package_path"` // e.g. "github.com/gin-gonic/gin@v1.9.1"
	FilePath    string       `json:"file_path,omitempty"` // e.g. "gin.go" (only for file cache)
}

type CacheKeyType string

const (
	CacheKeyTypePackage CacheKeyType = "package"
	CacheKeyTypeFile    CacheKeyType = "file"
)

// String returns a string representation of the cache key
func (k CacheKey) String() string {
	if k.Type == CacheKeyTypeFile {
		return fmt.Sprintf("file:%s:%s", k.PackagePath, k.FilePath)
	}
	return fmt.Sprintf("package:%s", k.PackagePath)
}

// CachedAnalysis represents a cached analysis result with revision tracking
type CachedAnalysis struct {
	// Revision identifier
	Revision string `json:"revision"`
	
	// Analysis results (one of these will be set)
	PackageInfo *PackageInfo `json:"package_info,omitempty"`
	FileInfo    *FileInfo    `json:"file_info,omitempty"`
	
	// Quality information
	Quality *AnalysisQuality `json:"quality"`
	
	// Metadata
	Timestamp            time.Time `json:"timestamp"`
	MissingDependencies  []string  `json:"missing_dependencies"`
	DependencyLoadingInProgress bool `json:"dependency_loading_in_progress"`
	
	// Complete analyses are kept indefinitely
	IsComplete bool `json:"is_complete"`
}

// AnalysisCache manages cached analysis results with revision-based updates
type AnalysisCache struct {
	cache map[string]*CachedAnalysis // key = CacheKey.String()
	mutex sync.RWMutex
	
	// Dependency checker for recalculation decisions
	dependencyChecker DependencyChecker
}

// DependencyChecker interface for checking dependency availability
type DependencyChecker interface {
	AreDependenciesAvailable(workDir string, dependencies []string) ([]string, error)
}

// NewAnalysisCache creates a new analysis cache
func NewAnalysisCache(dependencyChecker DependencyChecker) *AnalysisCache {
	return &AnalysisCache{
		cache:             make(map[string]*CachedAnalysis),
		dependencyChecker: dependencyChecker,
	}
}

// Get retrieves a cached analysis, considering the client's current revision
func (ac *AnalysisCache) Get(key CacheKey, clientRevision string) (*CachedAnalysis, CacheResult) {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	
	keyStr := key.String()
	cached, exists := ac.cache[keyStr]
	
	if !exists {
		return nil, CacheResultMiss
	}
	
	// If client has no revision (initial request), return cached version
	if clientRevision == "" {
		return cached, CacheResultHit
	}
	
	// If client has same revision as cached, no change
	if cached.Revision == clientRevision {
		return cached, CacheResultNoChange
	}
	
	// Client has different (older) revision, return newer cached version
	return cached, CacheResultNewer
}

// Set stores an analysis result in the cache
func (ac *AnalysisCache) Set(key CacheKey, analysis *CachedAnalysis) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	
	keyStr := key.String()
	
	// Remove previous revision if this is an update (unless previous was complete)
	if existing, exists := ac.cache[keyStr]; exists && !existing.IsComplete {
		// Replace with new revision
		ac.cache[keyStr] = analysis
	} else {
		// First time or previous was complete, just store
		ac.cache[keyStr] = analysis
	}
}

// ShouldRecalculate determines if we should recalculate analysis based on dependency availability
func (ac *AnalysisCache) ShouldRecalculate(key CacheKey, workDir string) (bool, []string, error) {
	ac.mutex.RLock()
	cached, exists := ac.cache[key.String()]
	ac.mutex.RUnlock()
	
	if !exists {
		return true, nil, nil // No cache, should calculate
	}
	
	if cached.IsComplete {
		return false, nil, nil // Complete analysis, no need to recalculate
	}
	
	if len(cached.MissingDependencies) == 0 {
		return false, nil, nil // No missing dependencies, no improvement possible
	}
	
	// Check if any previously missing dependencies are now available
	availableDeps, err := ac.dependencyChecker.AreDependenciesAvailable(workDir, cached.MissingDependencies)
	if err != nil {
		return false, nil, err
	}
	
	if len(availableDeps) > 0 {
		return true, availableDeps, nil // Some dependencies now available, should recalculate
	}
	
	return false, nil, nil // No new dependencies available
}

// MarkDependencyLoadingInProgress marks that dependency loading is in progress for a cache entry
func (ac *AnalysisCache) MarkDependencyLoadingInProgress(key CacheKey, inProgress bool) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	
	keyStr := key.String()
	if cached, exists := ac.cache[keyStr]; exists {
		cached.DependencyLoadingInProgress = inProgress
	}
}

// GetStats returns cache statistics
func (ac *AnalysisCache) GetStats() CacheStats {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	
	stats := CacheStats{
		TotalEntries: len(ac.cache),
	}
	
	for _, cached := range ac.cache {
		if cached.IsComplete {
			stats.CompleteEntries++
		} else {
			stats.IncompleteEntries++
		}
		
		if cached.DependencyLoadingInProgress {
			stats.LoadingInProgress++
		}
	}
	
	return stats
}

// Cleanup removes old incomplete entries (complete entries are kept)
func (ac *AnalysisCache) Cleanup(maxAge time.Duration) int {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	
	removed := 0
	now := time.Now()
	
	for keyStr, cached := range ac.cache {
		// Only remove incomplete entries that are old
		if !cached.IsComplete && now.Sub(cached.Timestamp) > maxAge {
			delete(ac.cache, keyStr)
			removed++
		}
	}
	
	return removed
}

// CacheResult represents the result of a cache lookup
type CacheResult string

const (
	CacheResultMiss     CacheResult = "miss"      // No cached entry
	CacheResultHit      CacheResult = "hit"       // Found cached entry
	CacheResultNoChange CacheResult = "no_change" // Client has same revision
	CacheResultNewer    CacheResult = "newer"     // Cache has newer revision
)

// CacheStats provides statistics about cache usage
type CacheStats struct {
	TotalEntries      int `json:"total_entries"`
	CompleteEntries   int `json:"complete_entries"`
	IncompleteEntries int `json:"incomplete_entries"`
	LoadingInProgress int `json:"loading_in_progress"`
}

// SimpleDependencyChecker implements basic dependency availability checking
type SimpleDependencyChecker struct{}

// AreDependenciesAvailable checks which dependencies are now available in the module cache
func (sdc *SimpleDependencyChecker) AreDependenciesAvailable(workDir string, dependencies []string) ([]string, error) {
	available := make([]string, 0)
	
	for _, dep := range dependencies {
		if isAvailable, err := sdc.checkSingleDependency(workDir, dep); err != nil {
			// Log error but continue checking other dependencies
			fmt.Printf("Error checking dependency %s: %v\n", dep, err)
		} else if isAvailable {
			available = append(available, dep)
		}
	}
	
	return available, nil
}

// checkSingleDependency checks if a single dependency is available
func (sdc *SimpleDependencyChecker) checkSingleDependency(workDir, dependency string) (bool, error) {
	// Use `go list` to check if the module can be resolved
	// This is faster than `go mod download` and doesn't modify the module cache
	cmd := exec.Command("go", "list", "-m", dependency)
	cmd.Dir = workDir
	
	// Set a timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, "go", "list", "-m", dependency)
	cmd.Dir = workDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If `go list` fails, the dependency is not available
		return false, nil
	}
	
	// If `go list` succeeds and outputs the module path, it's available
	outputStr := string(output)
	return len(outputStr) > 0 && !strings.Contains(outputStr, "not found"), nil
}