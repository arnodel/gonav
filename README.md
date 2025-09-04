# Go Navigator

A web-based code navigator for Go source code that allows you to browse repositories, view source files with syntax highlighting, and navigate between definitions.

## Features

- Load Go repositories from GitHub using `module@version` format
- Browse file structure of Go packages
- View source code with clickable symbols
- Navigate to symbol definitions within the same repository
- Cross-repository navigation (planned)

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

3. **Run the development environment:**
   ```bash
   ./dev.sh
   ```

   This will start:
   - Go backend server on `http://localhost:8080`
   - Parcel frontend dev server on `http://localhost:1234`

4. **Open your browser:**
   Go to `http://localhost:1234` and try loading a repository like:
   - `github.com/gin-gonic/gin@v1.9.1`
   - `github.com/gorilla/mux@v1.8.0`

### Manual Setup

If you prefer to run the servers separately:

**Backend:**
```bash
go run main.go
# Server runs on http://localhost:8080
```

**Frontend:**
```bash
cd frontend
npm run dev
# Dev server runs on http://localhost:1234
```

## Architecture

- **Backend (Go)**: REST API that clones repositories and parses Go source using `go/ast`
- **Frontend (React)**: Web interface for browsing and navigating code
- **Bundler (Parcel)**: Zero-config frontend build tool

## API Endpoints

- `GET /api/repo/{module@version}` - Load repository metadata and file list
- `GET /api/file/{module@version}/{file_path}` - Get parsed file content with symbols

## How It Works

1. Enter a Go module with version (e.g., `github.com/gin-gonic/gin@v1.9.1`)
2. Backend clones the repository to a temporary directory
3. Frontend displays the file tree
4. Click on files to view syntax-highlighted source code
5. Click on symbols to navigate to their definitions

## Current Limitations

- Only supports GitHub repositories
- Cross-repository navigation not yet implemented
- Basic symbol resolution (same file only)
- No semantic analysis or type information

## Next Steps

- [ ] Add support for GitLab, Bitbucket, and other Git hosts
- [ ] Implement cross-repository navigation using go.mod
- [ ] Add more sophisticated symbol resolution
- [ ] Implement search functionality
- [ ] Add syntax highlighting with Prism.js
- [ ] Support for viewing documentation
- [ ] Performance improvements for large repositories