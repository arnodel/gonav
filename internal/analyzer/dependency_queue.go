package analyzer

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// DependencyQueueConfig configures the dependency download queue
type DependencyQueueConfig struct {
	// MaxConcurrentDownloads limits how many downloads can happen simultaneously
	MaxConcurrentDownloads int
	
	// DownloadTimeout is the maximum time for a single dependency download
	DownloadTimeout time.Duration
	
	// QueueSize limits the number of pending download requests
	QueueSize int
	
	// RetryAttempts is the number of times to retry failed downloads
	RetryAttempts int
}

// DefaultDependencyQueueConfig returns sensible default configuration
func DefaultDependencyQueueConfig() DependencyQueueConfig {
	return DependencyQueueConfig{
		MaxConcurrentDownloads: 3,
		DownloadTimeout:        2 * time.Minute,
		QueueSize:             100,
		RetryAttempts:         2,
	}
}

// DependencyDownloadRequest represents a request to download dependencies
type DependencyDownloadRequest struct {
	WorkDir      string   `json:"work_dir"`
	Dependencies []string `json:"dependencies"`
	CacheKey     CacheKey `json:"cache_key"`
	RequestID    string   `json:"request_id"`
	
	// Response channel for completion notification
	ResultChan chan DependencyDownloadResult `json:"-"`
}

// DependencyDownloadResult represents the result of a dependency download operation
type DependencyDownloadResult struct {
	RequestID           string    `json:"request_id"`
	Successful          []string  `json:"successful"`
	Failed              []string  `json:"failed"`
	Errors              []string  `json:"errors,omitempty"`
	CompletedAt         time.Time `json:"completed_at"`
	TotalDownloadTime   time.Duration `json:"total_download_time"`
}

// DependencyQueue manages concurrent downloading of missing dependencies
type DependencyQueue struct {
	config    DependencyQueueConfig
	requests  chan DependencyDownloadRequest
	workers   []chan struct{} // Stop channels for workers
	active    map[string]bool // Track active downloads to prevent duplicates
	activeMux sync.RWMutex
	
	// Statistics
	stats     DependencyQueueStats
	statsMux  sync.RWMutex
	
	// Shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// DependencyQueueStats tracks queue performance
type DependencyQueueStats struct {
	TotalRequests     int64         `json:"total_requests"`
	CompletedRequests int64         `json:"completed_requests"`
	FailedRequests    int64         `json:"failed_requests"`
	ActiveDownloads   int           `json:"active_downloads"`
	QueueLength       int           `json:"queue_length"`
	AverageTime       time.Duration `json:"average_time"`
}

// NewDependencyQueue creates and starts a new dependency download queue
func NewDependencyQueue(config DependencyQueueConfig) *DependencyQueue {
	ctx, cancel := context.WithCancel(context.Background())
	
	dq := &DependencyQueue{
		config:     config,
		requests:   make(chan DependencyDownloadRequest, config.QueueSize),
		workers:    make([]chan struct{}, config.MaxConcurrentDownloads),
		active:     make(map[string]bool),
		ctx:        ctx,
		cancelFunc: cancel,
	}
	
	// Start worker goroutines
	for i := 0; i < config.MaxConcurrentDownloads; i++ {
		stopChan := make(chan struct{})
		dq.workers[i] = stopChan
		
		dq.wg.Add(1)
		go dq.worker(i, stopChan)
	}
	
	return dq
}

// SubmitDownloadRequest submits a dependency download request
func (dq *DependencyQueue) SubmitDownloadRequest(req DependencyDownloadRequest) error {
	// Check for duplicate active downloads
	dq.activeMux.RLock()
	key := req.CacheKey.String()
	if dq.active[key] {
		dq.activeMux.RUnlock()
		return fmt.Errorf("download already in progress for %s", key)
	}
	dq.activeMux.RUnlock()
	
	// Mark as active
	dq.activeMux.Lock()
	dq.active[key] = true
	dq.activeMux.Unlock()
	
	// Update stats
	dq.statsMux.Lock()
	dq.stats.TotalRequests++
	dq.stats.QueueLength = len(dq.requests)
	dq.statsMux.Unlock()
	
	// Submit to queue
	select {
	case dq.requests <- req:
		return nil
	case <-dq.ctx.Done():
		return fmt.Errorf("dependency queue is shutting down")
	default:
		// Queue is full
		dq.activeMux.Lock()
		delete(dq.active, key)
		dq.activeMux.Unlock()
		return fmt.Errorf("dependency queue is full")
	}
}

// worker processes download requests
func (dq *DependencyQueue) worker(workerID int, stopChan chan struct{}) {
	defer dq.wg.Done()
	
	for {
		select {
		case req := <-dq.requests:
			dq.processDownloadRequest(workerID, req)
		case <-stopChan:
			return
		case <-dq.ctx.Done():
			return
		}
	}
}

// processDownloadRequest handles a single download request
func (dq *DependencyQueue) processDownloadRequest(workerID int, req DependencyDownloadRequest) {
	startTime := time.Now()
	cacheKey := req.CacheKey.String()
	
	// Update active downloads count
	dq.statsMux.Lock()
	dq.stats.ActiveDownloads++
	dq.statsMux.Unlock()
	
	defer func() {
		// Clean up active tracking
		dq.activeMux.Lock()
		delete(dq.active, cacheKey)
		dq.activeMux.Unlock()
		
		// Update stats
		dq.statsMux.Lock()
		dq.stats.ActiveDownloads--
		dq.stats.CompletedRequests++
		dq.statsMux.Unlock()
	}()
	
	fmt.Printf("Worker %d: Starting download for %s (%d dependencies)\n", 
		workerID, cacheKey, len(req.Dependencies))
	
	result := DependencyDownloadResult{
		RequestID:   req.RequestID,
		Successful:  make([]string, 0),
		Failed:      make([]string, 0),
		Errors:      make([]string, 0),
		CompletedAt: time.Now(),
	}
	
	// Download each dependency
	for _, dep := range req.Dependencies {
		err := dq.downloadSingleDependency(req.WorkDir, dep)
		if err != nil {
			result.Failed = append(result.Failed, dep)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", dep, err))
			fmt.Printf("Worker %d: Failed to download %s: %v\n", workerID, dep, err)
		} else {
			result.Successful = append(result.Successful, dep)
			fmt.Printf("Worker %d: Successfully downloaded %s\n", workerID, dep)
		}
	}
	
	result.TotalDownloadTime = time.Since(startTime)
	
	// Send result if channel is provided
	if req.ResultChan != nil {
		select {
		case req.ResultChan <- result:
		default:
			// Channel full or closed, ignore
		}
	}
	
	fmt.Printf("Worker %d: Completed download for %s (success: %d, failed: %d, time: %v)\n",
		workerID, cacheKey, len(result.Successful), len(result.Failed), result.TotalDownloadTime)
}

// downloadSingleDependency downloads a single dependency using go mod download
func (dq *DependencyQueue) downloadSingleDependency(workDir, dependency string) error {
	ctx, cancel := context.WithTimeout(dq.ctx, dq.config.DownloadTimeout)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "go", "mod", "download", dependency)
	cmd.Dir = workDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod download failed: %w, output: %s", err, string(output))
	}
	
	return nil
}

// GetStats returns current queue statistics
func (dq *DependencyQueue) GetStats() DependencyQueueStats {
	dq.statsMux.RLock()
	defer dq.statsMux.RUnlock()
	
	stats := dq.stats
	stats.QueueLength = len(dq.requests) // Current queue length
	return stats
}

// Shutdown gracefully shuts down the dependency queue
func (dq *DependencyQueue) Shutdown(timeout time.Duration) error {
	fmt.Println("Shutting down dependency queue...")
	
	// Stop accepting new requests
	dq.cancelFunc()
	
	// Stop workers
	for _, stopChan := range dq.workers {
		close(stopChan)
	}
	
	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		dq.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		fmt.Println("Dependency queue shutdown completed")
		return nil
	case <-time.After(timeout):
		fmt.Println("Dependency queue shutdown timed out")
		return fmt.Errorf("shutdown timed out after %v", timeout)
	}
}

// IsActive returns true if a download is currently in progress for the given cache key
func (dq *DependencyQueue) IsActive(key CacheKey) bool {
	dq.activeMux.RLock()
	defer dq.activeMux.RUnlock()
	
	return dq.active[key.String()]
}