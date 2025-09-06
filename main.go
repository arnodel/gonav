package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gonav/internal/analyzer"
	"gonav/internal/repo"
)

type Server struct {
	repoManager   *repo.Manager
	analyzer      *analyzer.PackageAnalyzer
	// Cache for package discoveries per repository
	discoveryCache map[string]map[string]*analyzer.PackageDiscovery
}

func NewServer() *Server {
	return &Server{
		repoManager:    repo.NewManager(),
		analyzer:       analyzer.New(),
		discoveryCache: make(map[string]map[string]*analyzer.PackageDiscovery),
	}
}

func (s *Server) handleRepo(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract module@version from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/repo/")
	moduleAtVersion, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, "Invalid module format", http.StatusBadRequest)
		return
	}

	fmt.Printf("Loading repository: '%s'\n", moduleAtVersion)
	fmt.Printf("Raw URL path was: '%s'\n", strings.TrimPrefix(r.URL.Path, "/api/repo/"))

	// Load repository
	repoInfo, err := s.repoManager.LoadRepository(moduleAtVersion)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load repository: %v", err), http.StatusInternalServerError)
		return
	}

	// Discover packages in the repository (fast operation)
	repoPath := s.repoManager.GetRepositoryPath(moduleAtVersion)
	if repoPath != "" {
		packageDiscoveries, err := s.analyzer.DiscoverPackages(repoPath)
		if err != nil {
			fmt.Printf("Failed to discover packages (continuing anyway): %v\n", err)
		} else {
			s.discoveryCache[moduleAtVersion] = packageDiscoveries
			fmt.Printf("Successfully discovered %d packages\n", len(packageDiscoveries))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repoInfo)
}

func (s *Server) handlePackage(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract module@version and package path from URL
	// URL format: /api/package/{module@version}/{package_path}
	path := strings.TrimPrefix(r.URL.Path, "/api/package/")
	
	// First, let's URL decode the entire path
	decodedPath, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, "Invalid URL encoding", http.StatusBadRequest)
		return
	}
	
	// Now we have something like: github.com/owner/repo@version/package/path
	// We need to find where the module@version ends and the package path begins
	// Look for the @ symbol to find the version, then find the next / after that
	
	atIndex := strings.Index(decodedPath, "@")
	if atIndex == -1 {
		http.Error(w, "Invalid module@version format", http.StatusBadRequest)
		return
	}
	
	// Find the first / after the @version part
	versionStart := atIndex + 1
	slashAfterVersion := strings.Index(decodedPath[versionStart:], "/")
	
	var moduleAtVersion string
	var packagePath string
	
	if slashAfterVersion == -1 {
		// No package path, just module@version
		moduleAtVersion = decodedPath
		packagePath = ""
	} else {
		moduleAtVersionEnd := versionStart + slashAfterVersion
		moduleAtVersion = decodedPath[:moduleAtVersionEnd]
		packagePath = decodedPath[moduleAtVersionEnd+1:]
	}

	fmt.Printf("Analyzing package: '%s' in repository: '%s'\n", packagePath, moduleAtVersion)

	// Get repository path
	repoPath := s.repoManager.GetRepositoryPath(moduleAtVersion)
	if repoPath == "" {
		fmt.Printf("Repository not found for: '%s'\n", moduleAtVersion)
		http.Error(w, "Repository not loaded", http.StatusNotFound)
		return
	}

	// Analyze the specific package
	packageInfo, err := s.analyzer.AnalyzePackage(repoPath, packagePath)
	if err != nil {
		fmt.Printf("Failed to analyze package: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to analyze package: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Successfully analyzed package with %d symbols and %d files\n", 
		len(packageInfo.Symbols), len(packageInfo.Files))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(packageInfo)
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract module@version and file path from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/file/")
	
	// The path will be like: github.com%2Fowner%2Frepo%40version/path/to/file.go
	// We need to find the first unescaped '/' to split module from file path
	
	// First, let's URL decode the entire path
	decodedPath, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, "Invalid URL encoding", http.StatusBadRequest)
		return
	}
	
	// Now we have something like: github.com/owner/repo@version/path/to/file.go
	// We need to find where the module@version ends and the file path begins
	// Look for the @ symbol to find the version, then find the next / after that
	
	atIndex := strings.Index(decodedPath, "@")
	if atIndex == -1 {
		http.Error(w, "Invalid module@version format", http.StatusBadRequest)
		return
	}
	
	// Find the first / after the @version part
	versionStart := atIndex + 1
	slashAfterVersion := strings.Index(decodedPath[versionStart:], "/")
	if slashAfterVersion == -1 {
		http.Error(w, "Invalid file path format", http.StatusBadRequest)
		return
	}
	
	moduleAtVersionEnd := versionStart + slashAfterVersion
	moduleAtVersion := decodedPath[:moduleAtVersionEnd]
	filePath := decodedPath[moduleAtVersionEnd+1:]

	fmt.Printf("Loading file: '%s' from repository: '%s'\n", filePath, moduleAtVersion)

	// Get repository path
	repoPath := s.repoManager.GetRepositoryPath(moduleAtVersion)
	if repoPath == "" {
		fmt.Printf("Repository not found for: '%s'\n", moduleAtVersion)
		fmt.Printf("Available repositories: %v\n", s.repoManager.ListRepositories())
		http.Error(w, "Repository not loaded", http.StatusNotFound)
		return
	}

	fmt.Printf("Repository path: '%s'\n", repoPath)

	// Parse file
	fullPath := filepath.Join(repoPath, filePath)
	fmt.Printf("Attempting to parse file at: '%s'\n", fullPath)
	
	// Analyze the specific file
	analyzerFileInfo, err := s.analyzer.AnalyzeSingleFile(repoPath, filePath)
	if err != nil {
		fmt.Printf("Failed to analyze file %s: %v\n", filePath, err)
	} else {
		fmt.Printf("Returning analyzed file info with %d symbols and %d references\n", 
			len(analyzerFileInfo.Symbols), len(analyzerFileInfo.References))
		
		// Convert analyzer format to frontend-expected format
		frontendFileInfo := map[string]interface{}{
			"source": analyzerFileInfo.Source,
			"symbols": make(map[string]interface{}),
			"references": analyzerFileInfo.References,
		}
		
		// Convert symbols to the expected format
		for _, symbol := range analyzerFileInfo.Symbols {
			frontendFileInfo["symbols"].(map[string]interface{})[symbol.Name] = map[string]interface{}{
				"name": symbol.Name,
				"type": symbol.Type,
				"file": symbol.File,
				"line": symbol.Line,
				"package": symbol.Package,
			}
		}
		
		fmt.Printf("Converted to frontend format with %d symbols\n", 
			len(frontendFileInfo["symbols"].(map[string]interface{})))
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(frontendFileInfo)
		return
	}

	// Fallback: read file manually if not in analysis
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		fmt.Printf("File does not exist at: '%s'\n", fullPath)
		
		// List files in the directory for debugging
		dir := filepath.Dir(fullPath)
		fmt.Printf("Files in directory '%s':\n", dir)
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				fmt.Printf("  - %s\n", entry.Name())
			}
		}
		
		http.Error(w, fmt.Sprintf("File not found: %s", filePath), http.StatusNotFound)
		return
	}

	// Simple file content without analysis
	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	// Return basic file info without cross-references
	basicFileInfo := map[string]interface{}{
		"source":     string(content),
		"symbols":    make(map[string]interface{}),
		"references": make([]interface{}, 0),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(basicFileInfo)
}


func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/repo/", s.handleRepo)
	mux.HandleFunc("/api/package/", s.handlePackage)
	mux.HandleFunc("/api/file/", s.handleFile)

	// Serve static files for development
	mux.Handle("/", http.FileServer(http.Dir("frontend/dist")))

	return mux
}

func main() {
	server := NewServer()
	mux := server.setupRoutes()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Channel to listen for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		fmt.Printf("Server starting on port %s\n", port)
		fmt.Printf("Frontend will be served from: frontend/dist\n")
		fmt.Printf("API available at: /api/repo/{module@version} and /api/file/{module@version}/{path}\n")
		
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start:", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	fmt.Println("Shutting down server...")

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	fmt.Println("Server stopped gracefully")
}