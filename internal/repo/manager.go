package repo

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Manager struct {
	cacheDir string
	repos    map[string]string // moduleAtVersion -> local path
}

type RepositoryInfo struct {
	ModuleAtVersion string      `json:"moduleAtVersion"`
	ModulePath      string      `json:"modulePath"`
	Version         string      `json:"version"`
	Files           []FileInfo  `json:"files"`
}

type FileInfo struct {
	Path string `json:"path"`
	IsGo bool   `json:"isGo"`
}

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

func NewManager() *Manager {
	cacheDir := filepath.Join(os.TempDir(), "gonav-cache")
	os.MkdirAll(cacheDir, 0755)

	return &Manager{
		cacheDir: cacheDir,
		repos:    make(map[string]string),
	}
}

func (m *Manager) LoadRepository(moduleAtVersion string) (*RepositoryInfo, error) {
	// Check if already loaded
	if localPath, exists := m.repos[moduleAtVersion]; exists {
		return m.buildRepositoryInfo(moduleAtVersion, localPath)
	}

	// Parse module@version
	modulePath, version := m.parseModuleAtVersion(moduleAtVersion)
	if modulePath == "" {
		return nil, fmt.Errorf("invalid module@version format: %s", moduleAtVersion)
	}

	// Create local path for this repo
	safeName := strings.ReplaceAll(moduleAtVersion, "/", "_")
	safeName = strings.ReplaceAll(safeName, "@", "_")
	localPath := filepath.Join(m.cacheDir, safeName)

	// Clone or download the repository
	err := m.downloadRepository(modulePath, version, localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to download repository: %w", err)
	}

	// Store in cache
	m.repos[moduleAtVersion] = localPath

	return m.buildRepositoryInfo(moduleAtVersion, localPath)
}

func (m *Manager) GetRepositoryPath(moduleAtVersion string) string {
	return m.repos[moduleAtVersion]
}

func (m *Manager) ListRepositories() []string {
	var repos []string
	for key := range m.repos {
		repos = append(repos, key)
	}
	return repos
}

func (m *Manager) parseModuleAtVersion(moduleAtVersion string) (string, string) {
	parts := strings.Split(moduleAtVersion, "@")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func (m *Manager) downloadRepository(modulePath, version, localPath string) error {
	// Try go mod download first (preferred method for Go modules)
	localDir, err := m.downloadWithGoMod(modulePath, version)
	if err == nil {
		// Success with go mod download, create a symlink or copy to our expected location
		os.RemoveAll(localPath)
		return os.Symlink(localDir, localPath)
	}

	// Fall back to git clone for modules not available via go proxy
	fmt.Printf("go mod download failed for %s@%s: %v, trying git clone...\n", modulePath, version, err)
	
	// Remove existing directory if it exists
	os.RemoveAll(localPath)

	// Use git clone as fallback
	if strings.HasPrefix(modulePath, "github.com/") {
		return m.cloneGitRepository(modulePath, version, localPath)
	}

	return fmt.Errorf("unsupported module path and go mod download failed: %s", modulePath)
}

func (m *Manager) downloadWithGoMod(modulePath, version string) (string, error) {
	moduleAtVersion := modulePath + "@" + version
	
	// Use go mod download with JSON output to get the exact location
	cmd := exec.Command("go", "mod", "download", "-json", moduleAtVersion)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go mod download failed: %w", err)
	}

	var downloadInfo GoModDownloadInfo
	if err := json.Unmarshal(output, &downloadInfo); err != nil {
		return "", fmt.Errorf("failed to parse go mod download output: %w", err)
	}

	// Verify the directory exists
	if downloadInfo.Dir == "" {
		return "", fmt.Errorf("go mod download did not provide directory path")
	}

	if _, err := os.Stat(downloadInfo.Dir); os.IsNotExist(err) {
		return "", fmt.Errorf("go mod download directory does not exist: %s", downloadInfo.Dir)
	}

	return downloadInfo.Dir, nil
}

func (m *Manager) cloneGitRepository(modulePath, version, localPath string) error {
	// Convert module path to git URL
	gitURL := fmt.Sprintf("https://%s.git", modulePath)

	// Clone the repository
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", version, gitURL, localPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If branch doesn't exist, try cloning without branch and then checkout
		cmd = exec.Command("git", "clone", "--depth", "1", gitURL, localPath)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git clone failed: %s", string(output))
		}

		// Try to checkout the version
		cmd = exec.Command("git", "-C", localPath, "fetch", "--depth", "1", "origin", version)
		cmd.Run() // Ignore error, might be a tag

		cmd = exec.Command("git", "-C", localPath, "checkout", version)
		output, err = cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Warning: could not checkout version %s: %s\n", version, string(output))
		}
	}

	return nil
}

func (m *Manager) buildRepositoryInfo(moduleAtVersion, localPath string) (*RepositoryInfo, error) {
	modulePath, version := m.parseModuleAtVersion(moduleAtVersion)
	
	// Find all Go files
	files, err := m.findGoFiles(localPath)
	if err != nil {
		return nil, err
	}

	return &RepositoryInfo{
		ModuleAtVersion: moduleAtVersion,
		ModulePath:      modulePath,
		Version:         version,
		Files:           files,
	}, nil
}

func (m *Manager) findGoFiles(rootPath string) ([]FileInfo, error) {
	var files []FileInfo

	// Resolve symlinks to the actual path
	resolvedPath, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		// If symlink resolution fails, use the original path
		resolvedPath = rootPath
	}

	err = filepath.Walk(resolvedPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and files
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip vendor directory
		if info.IsDir() && info.Name() == "vendor" {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			// Get relative path from the resolved root
			relPath, err := filepath.Rel(resolvedPath, path)
			if err != nil {
				return err
			}

			// Convert to forward slashes for consistency
			relPath = filepath.ToSlash(relPath)

			isGo := strings.HasSuffix(relPath, ".go")
			files = append(files, FileInfo{
				Path: relPath,
				IsGo: isGo,
			})
		}

		return nil
	})

	return files, err
}