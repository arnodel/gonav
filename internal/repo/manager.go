package repo

import (
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
	// Remove existing directory if it exists
	os.RemoveAll(localPath)

	// For now, we'll use git clone for GitHub repositories
	// In a production system, you'd want to handle different VCS systems
	if strings.HasPrefix(modulePath, "github.com/") {
		return m.cloneGitRepository(modulePath, version, localPath)
	}

	return fmt.Errorf("unsupported module path: %s", modulePath)
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

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
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
			// Get relative path
			relPath, err := filepath.Rel(rootPath, path)
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