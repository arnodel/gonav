package analyzer

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// GenerateRevision creates a revision identifier based on the analysis state
// The revision changes when the analysis becomes more complete or accurate
func GenerateRevision(packagePath string, quality *AnalysisQuality, symbolCount int, refCount int) string {
	h := sha256.New()
	
	// Include the package path
	h.Write([]byte(packagePath))
	
	// Include analysis completeness state
	h.Write([]byte(fmt.Sprintf("complete:%v", quality.IsComplete)))
	h.Write([]byte(fmt.Sprintf("mode:%s", quality.AnalysisMode)))
	h.Write([]byte(fmt.Sprintf("score:%.3f", quality.QualityScore)))
	
	// Include missing dependencies (sorted for consistency)
	sortedDeps := make([]string, len(quality.MissingDependencies))
	copy(sortedDeps, quality.MissingDependencies)
	sort.Strings(sortedDeps)
	h.Write([]byte(fmt.Sprintf("missing:%s", strings.Join(sortedDeps, ","))))
	
	// Include symbol and reference counts as they reflect analysis depth
	h.Write([]byte(fmt.Sprintf("symbols:%d", symbolCount)))
	h.Write([]byte(fmt.Sprintf("refs:%d", refCount)))
	
	// Return short hex hash
	hash := h.Sum(nil)
	return fmt.Sprintf("%x", hash[:8]) // 16 character hex string
}

// CompareRevisions returns true if revisionA represents newer/better analysis than revisionB
func CompareRevisions(revisionA, revisionB string) bool {
	// For now, simple string comparison (different = potentially newer)
	// In future, could implement more sophisticated comparison
	return revisionA != revisionB
}

// RevisionInfo contains metadata about an analysis revision
type RevisionInfo struct {
	Revision  string `json:"revision"`
	Complete  bool   `json:"complete"`
	Quality   float64 `json:"quality,omitempty"`   // Optional quality score
	NoChange  bool   `json:"no_change,omitempty"` // Set to true when client has latest revision
}

// CreateRevisionInfo creates revision metadata from quality assessment
func CreateRevisionInfo(revision string, quality *AnalysisQuality) RevisionInfo {
	return RevisionInfo{
		Revision: revision,
		Complete: quality.IsComplete,
		Quality:  quality.QualityScore,
		NoChange: false,
	}
}