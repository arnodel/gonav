# Go Navigator

A web-based code navigator for Go source code that allows you to browse repositories, view source files with syntax highlighting, and navigate between definitions.

## Features

- Load Go repositories from GitHub using `module@version` format
- Browse file structure of Go packages
- View source code with clickable symbols
- Navigate to symbol definitions within the same repository
- **Cross-repository navigation** - Navigate to external modules and dependencies
- **Enhanced analysis** using `golang.org/x/tools/go/packages` for accurate type information
- **Isolated environments** to prevent dependency conflicts
- **Standard library detection** with proper classification

## Quick Start

### Prerequisites

- Go 1.19 or later
- Node.js and npm
- Git (for cloning repositories)

### Development Setup

1. **Clone the project:**
   ```bash
   git clone <your-repo-url>
   cd gonav
   ```

2. **Install frontend dependencies:**
   ```bash
   cd frontend
   npm install
   cd ..
   ```

3. **Start the development servers:**

   You'll need two terminal windows:

   **Terminal 1 - Backend:**
   ```bash
   make backend
   # Or: go run main.go
   # Server runs on http://localhost:8080
   ```

   **Terminal 2 - Frontend:**
   ```bash
   make frontend
   # Or: cd frontend && npm run dev
   # Dev server runs on http://localhost:1234
   ```

4. **Open your browser:**
   Go to `http://localhost:1234` and try loading a repository like:
   - `github.com/gin-gonic/gin@v1.9.1`
   - `github.com/gorilla/mux@v1.8.0`

### Available Commands

Run `make help` to see all available commands:

```bash
make help      # Show help
make backend   # Start Go backend server (port 8080)
make frontend  # Start frontend dev server (port 1234)
make build     # Build Go binary
make clean     # Clean build artifacts
```

## Architecture

- **Backend (Go)**: REST API that clones repositories in isolated environments and parses Go source using `golang.org/x/tools/go/packages` for enhanced analysis
- **Frontend (React)**: Web interface for browsing and navigating code with cross-repository support
- **Bundler (Parcel)**: Zero-config frontend build tool

## API Endpoints

- `GET /api/repo/{module@version}` - Load repository metadata and file list
- `GET /api/file/{module@version}/{file_path}` - Get parsed file content with symbols

## How It Works

1. Enter a Go module with version (e.g., `github.com/gin-gonic/gin@v1.9.1`)
2. Backend clones the repository to an isolated directory to avoid dependency conflicts
3. Enhanced analysis using `golang.org/x/tools/go/packages` provides accurate type information
4. Frontend displays the file tree with full navigation support
5. Click on files to view syntax-highlighted source code
6. Click on symbols to navigate to their definitions (same-repo or cross-repository)

## Advanced Features

- **Isolated Environments**: Each repository is analyzed in its own Go module cache to prevent conflicts
- **Enhanced Analysis**: Uses `golang.org/x/tools/go/packages` for precise symbol resolution
- **Cross-Repository Navigation**: Click on external dependencies to navigate to their source code
- **Standard Library Support**: Proper detection and handling of Go standard library symbols
- **Module Resolution**: Automatic resolution of module@version references for external navigation

## Next Steps

- [ ] Add support for GitLab, Bitbucket, and other Git hosts
- [ ] Implement search functionality across repositories
- [ ] Add syntax highlighting with Prism.js  
- [ ] Support for viewing documentation and comments
- [ ] Performance improvements for large repositories
- [ ] Caching and persistence for analyzed repositories