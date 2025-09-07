package env

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// IsolatedEnv provides an isolated Go environment for module operations
type IsolatedEnv struct {
	BaseDir    string
	GoModCache string
	GoCache    string
	GoPath     string
	env        []string
}

// GoModDownloadInfo represents the JSON output from 'go mod download -json'
type GoModDownloadInfo struct {
	Path     string `json:"Path"`     // Module path (e.g., "github.com/arnodel/edit")
	Version  string `json:"Version"`  // Resolved version (e.g., "v0.0.0-20220202110212-dfc8d7a13890")
	Info     string `json:"Info"`     // Path to .info file
	GoMod    string `json:"GoMod"`    // Path to .mod file
	Zip      string `json:"Zip"`      // Path to .zip file
	Dir      string `json:"Dir"`      // Path to cached source directory
	Sum      string `json:"Sum"`      // Checksum
	GoModSum string `json:"GoModSum"` // GoMod checksum
}

// NewIsolated creates a new isolated Go environment
func NewIsolated(baseDir string) (*IsolatedEnv, error) {
	env := &IsolatedEnv{
		BaseDir:    baseDir,
		GoModCache: filepath.Join(baseDir, "gomodcache"),
		GoCache:    filepath.Join(baseDir, "gocache"),
		GoPath:     filepath.Join(baseDir, "gopath"),
	}

	// Create directories
	if err := os.MkdirAll(env.GoModCache, 0755); err != nil {
		return nil, fmt.Errorf("failed to create gomodcache directory: %w", err)
	}
	if err := os.MkdirAll(env.GoCache, 0755); err != nil {
		return nil, fmt.Errorf("failed to create gocache directory: %w", err)
	}
	if err := os.MkdirAll(env.GoPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create gopath directory: %w", err)
	}

	// Setup environment variables
	env.env = append(os.Environ(),
		fmt.Sprintf("GOMODCACHE=%s", env.GoModCache),
		fmt.Sprintf("GOCACHE=%s", env.GoCache),
		fmt.Sprintf("GOPATH=%s", env.GoPath),
		"GO111MODULE=on",
	)

	return env, nil
}

// Environment returns the environment variables for this isolated environment
func (e *IsolatedEnv) Environment() []string {
	return e.env
}

// ExecCommand creates a command that will run in this isolated environment
func (e *IsolatedEnv) ExecCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = e.env
	return cmd
}

// DownloadModule downloads a module to the isolated cache and returns the directory path
func (e *IsolatedEnv) DownloadModule(moduleAtVersion string) (*GoModDownloadInfo, error) {
	cmd := e.ExecCommand("go", "mod", "download", "-json", moduleAtVersion)
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go mod download failed for %s: %w", moduleAtVersion, err)
	}

	var downloadInfo GoModDownloadInfo
	if err := json.Unmarshal(output, &downloadInfo); err != nil {
		return nil, fmt.Errorf("failed to parse go mod download output: %w", err)
	}

	// Verify the directory exists
	if downloadInfo.Dir == "" {
		return nil, fmt.Errorf("go mod download did not provide directory path")
	}

	if _, err := os.Stat(downloadInfo.Dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("go mod download directory does not exist: %s", downloadInfo.Dir)
	}

	return &downloadInfo, nil
}

// ModuleCachePath returns the path to a specific module in the cache
func (e *IsolatedEnv) ModuleCachePath(modulePath, version string) string {
	return filepath.Join(e.GoModCache, modulePath+"@"+version)
}

// Cleanup removes the isolated environment directory
func (e *IsolatedEnv) Cleanup() error {
	// Go module cache may contain read-only files, so we need to make them writable first
	err := filepath.Walk(e.BaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Make files and directories writable so they can be removed
		return os.Chmod(path, 0755)
	})
	if err != nil {
		// If chmod fails, continue with removal anyway
		fmt.Printf("Warning: failed to make files writable during cleanup: %v\n", err)
	}
	
	return os.RemoveAll(e.BaseDir)
}

// Stats returns information about the isolated environment
func (e *IsolatedEnv) Stats() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["base_dir"] = e.BaseDir
	stats["gomodcache"] = e.GoModCache
	stats["gocache"] = e.GoCache
	stats["gopath"] = e.GoPath
	
	// Count modules in cache
	if entries, err := os.ReadDir(e.GoModCache); err == nil {
		moduleCount := 0
		for _, entry := range entries {
			if entry.IsDir() && filepath.Base(entry.Name()) != "cache" {
				moduleCount++
			}
		}
		stats["cached_modules"] = moduleCount
	}
	
	return stats
}