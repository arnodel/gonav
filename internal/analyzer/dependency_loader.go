package analyzer

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// DependencyLoader handles asynchronous loading of missing dependencies
type DependencyLoader struct {
	// activeJobs tracks currently running dependency loading jobs
	activeJobs map[string]*LoadingJob
	jobsMutex  sync.RWMutex
	
	// workDir is the directory where go mod download should be executed
	workDir string
	
	// environment variables for go commands
	env []string
}

// LoadingJob represents a background dependency loading operation
type LoadingJob struct {
	ID            string                   `json:"id"`
	Dependencies  []string                 `json:"dependencies"`
	Status        LoadingStatus           `json:"status"`
	Progress      DependencyProgress      `json:"progress"`
	StartTime     time.Time               `json:"start_time"`
	CompletedTime *time.Time              `json:"completed_time,omitempty"`
	Loaded        []string                `json:"loaded"`
	Failed        []string                `json:"failed"`
	Errors        []string                `json:"errors,omitempty"`
	
	// Internal fields
	ctx        context.Context
	cancelFunc context.CancelFunc
	updates    chan DependencyProgress
}

// NewDependencyLoader creates a new dependency loader
func NewDependencyLoader(workDir string, env []string) *DependencyLoader {
	return &DependencyLoader{
		activeJobs: make(map[string]*LoadingJob),
		workDir:    workDir,
		env:        env,
	}
}

// StartDependencyLoading initiates background loading of missing dependencies
func (dl *DependencyLoader) StartDependencyLoading(enhancementToken string, missingDeps []string) (*LoadingJob, error) {
	dl.jobsMutex.Lock()
	defer dl.jobsMutex.Unlock()
	
	// Check if already loading this token
	if existingJob, exists := dl.activeJobs[enhancementToken]; exists {
		return existingJob, nil
	}
	
	// Create new loading job
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute) // 10 minute timeout
	job := &LoadingJob{
		ID:           enhancementToken,
		Dependencies: missingDeps,
		Status:       LoadingStatusInProgress,
		Progress: DependencyProgress{
			Total:     len(missingDeps),
			Completed: 0,
			Failed:    0,
		},
		StartTime:  time.Now(),
		Loaded:     make([]string, 0),
		Failed:     make([]string, 0),
		Errors:     make([]string, 0),
		ctx:        ctx,
		cancelFunc: cancel,
		updates:    make(chan DependencyProgress, len(missingDeps)),
	}
	
	dl.activeJobs[enhancementToken] = job
	
	// Start background loading
	go dl.runDependencyLoading(job)
	
	return job, nil
}

// GetLoadingStatus returns the current status of a dependency loading job
func (dl *DependencyLoader) GetLoadingStatus(enhancementToken string) (*DependencyLoadingStatus, error) {
	dl.jobsMutex.RLock()
	defer dl.jobsMutex.RUnlock()
	
	job, exists := dl.activeJobs[enhancementToken]
	if !exists {
		return &DependencyLoadingStatus{
			Status:   LoadingStatusIdle,
			Progress: DependencyProgress{},
		}, nil
	}
	
	estimatedCompletion := ""
	if job.Status == LoadingStatusInProgress && job.Progress.Completed > 0 {
		elapsed := time.Since(job.StartTime)
		avgTimePerDep := elapsed / time.Duration(job.Progress.Completed)
		remaining := job.Progress.Total - job.Progress.Completed
		estimatedCompletion = fmt.Sprintf("~%v", avgTimePerDep*time.Duration(remaining))
	}
	
	return &DependencyLoadingStatus{
		Status:              job.Status,
		Progress:            job.Progress,
		EstimatedCompletion: estimatedCompletion,
		LoadedDependencies:  job.Loaded,
		FailedDependencies:  job.Failed,
	}, nil
}

// CancelLoading cancels a running dependency loading job
func (dl *DependencyLoader) CancelLoading(enhancementToken string) error {
	dl.jobsMutex.Lock()
	defer dl.jobsMutex.Unlock()
	
	job, exists := dl.activeJobs[enhancementToken]
	if !exists {
		return fmt.Errorf("no loading job found for token: %s", enhancementToken)
	}
	
	job.cancelFunc()
	job.Status = LoadingStatusFailed
	delete(dl.activeJobs, enhancementToken)
	
	return nil
}

// runDependencyLoading executes the actual dependency loading in background
func (dl *DependencyLoader) runDependencyLoading(job *LoadingJob) {
	defer func() {
		close(job.updates)
		dl.jobsMutex.Lock()
		delete(dl.activeJobs, job.ID)
		dl.jobsMutex.Unlock()
	}()
	
	fmt.Printf("Starting dependency loading for job %s: %v\n", job.ID, job.Dependencies)
	
	for _, dep := range job.Dependencies {
		select {
		case <-job.ctx.Done():
			// Job was cancelled
			job.Status = LoadingStatusFailed
			now := time.Now()
			job.CompletedTime = &now
			return
		default:
			// Load this dependency
			err := dl.loadSingleDependency(dep)
			if err != nil {
				job.Failed = append(job.Failed, dep)
				job.Errors = append(job.Errors, fmt.Sprintf("%s: %v", dep, err))
				job.Progress.Failed++
				fmt.Printf("Failed to load dependency %s: %v\n", dep, err)
			} else {
				job.Loaded = append(job.Loaded, dep)
				job.Progress.Completed++
				fmt.Printf("Successfully loaded dependency: %s\n", dep)
			}
			
			// Send progress update
			select {
			case job.updates <- job.Progress:
			default:
				// Channel full, skip update
			}
		}
	}
	
	// Determine final status
	if len(job.Failed) == 0 {
		job.Status = LoadingStatusComplete
	} else if len(job.Loaded) == 0 {
		job.Status = LoadingStatusFailed
	} else {
		job.Status = LoadingStatusComplete // Partial success is still complete
	}
	
	now := time.Now()
	job.CompletedTime = &now
	
	fmt.Printf("Dependency loading completed for job %s: loaded=%d, failed=%d\n", 
		job.ID, len(job.Loaded), len(job.Failed))
}

// loadSingleDependency downloads a single dependency using go mod download
func (dl *DependencyLoader) loadSingleDependency(dependency string) error {
	// Execute go mod download for this specific dependency
	cmd := exec.Command("go", "mod", "download", dependency)
	cmd.Dir = dl.workDir
	
	// Set environment
	if dl.env != nil {
		cmd.Env = dl.env
	}
	
	// Run the command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	cmd = exec.CommandContext(ctx, "go", "mod", "download", dependency)
	cmd.Dir = dl.workDir
	if dl.env != nil {
		cmd.Env = dl.env
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod download failed: %w, output: %s", err, string(output))
	}
	
	return nil
}

// GetProgressUpdates returns a channel for receiving real-time progress updates
func (dl *DependencyLoader) GetProgressUpdates(enhancementToken string) (<-chan DependencyProgress, error) {
	dl.jobsMutex.RLock()
	defer dl.jobsMutex.RUnlock()
	
	job, exists := dl.activeJobs[enhancementToken]
	if !exists {
		return nil, fmt.Errorf("no loading job found for token: %s", enhancementToken)
	}
	
	return job.updates, nil
}

// CleanupCompletedJobs removes completed jobs older than the specified duration
func (dl *DependencyLoader) CleanupCompletedJobs(maxAge time.Duration) {
	dl.jobsMutex.Lock()
	defer dl.jobsMutex.Unlock()
	
	now := time.Now()
	for token, job := range dl.activeJobs {
		if job.CompletedTime != nil && now.Sub(*job.CompletedTime) > maxAge {
			delete(dl.activeJobs, token)
		}
	}
}

// ListActiveJobs returns information about all active loading jobs
func (dl *DependencyLoader) ListActiveJobs() []*LoadingJob {
	dl.jobsMutex.RLock()
	defer dl.jobsMutex.RUnlock()
	
	jobs := make([]*LoadingJob, 0, len(dl.activeJobs))
	for _, job := range dl.activeJobs {
		// Create a copy to avoid race conditions
		jobCopy := &LoadingJob{
			ID:            job.ID,
			Dependencies:  job.Dependencies,
			Status:        job.Status,
			Progress:      job.Progress,
			StartTime:     job.StartTime,
			CompletedTime: job.CompletedTime,
			Loaded:        job.Loaded,
			Failed:        job.Failed,
			Errors:        job.Errors,
		}
		jobs = append(jobs, jobCopy)
	}
	
	return jobs
}

// SetDependencyLoader configures the packages analyzer with dependency loading support
func (pa *PackagesAnalyzer) SetDependencyLoader(loader *DependencyLoader) {
	pa.dependencyLoader = loader
}

// TriggerEnhancedAnalysis starts dependency loading and returns an enhanced analysis response
func (pa *PackagesAnalyzer) TriggerEnhancedAnalysis(packagePath string) (*EnhancedAnalysisResponse, error) {
	// First, do initial analysis
	response, err := pa.AnalyzePackageWithQuality(packagePath)
	if err != nil {
		return response, err
	}
	
	// If enhancement is available and we have a dependency loader, start loading
	if response.Quality.EnhancementAvailable && pa.dependencyLoader != nil {
		job, err := pa.dependencyLoader.StartDependencyLoading(
			response.EnhancementToken, 
			response.Quality.MissingDependencies,
		)
		if err != nil {
			// Log error but don't fail the response
			fmt.Printf("Failed to start dependency loading: %v\n", err)
		} else {
			// Add dependency status to response
			response.DependencyStatus = &DependencyLoadingStatus{
				Status:   job.Status,
				Progress: job.Progress,
			}
		}
	}
	
	return response, nil
}