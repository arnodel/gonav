# Go Navigator API Documentation

The Go Navigator backend provides a REST API for loading and analyzing Go source code repositories.

## Base URL

```
http://localhost:8080/api
```

## Endpoints

### 1. Load Repository

Load a Go module repository and discover its packages.

**Endpoint:** `GET /repo/{moduleAtVersion}`

**Parameters:**
- `moduleAtVersion` (path): URL-encoded module name with version (e.g., `github.com%2Fowner%2Frepo%40v1.0.0`)

**Example Request:**
```bash
curl "http://localhost:8080/api/repo/github.com%2Farnodel%2Fgolua%40v0.1.0"
```

**Response:**
```json
{
  "moduleAtVersion": "github.com/arnodel/golua@v0.1.0",
  "modulePath": "github.com/arnodel/golua", 
  "version": "v0.1.0",
  "files": [
    {
      "path": "main.go",
      "isGo": true
    },
    {
      "path": "README.md", 
      "isGo": false
    }
  ]
}
```

**Response Fields:**
- `moduleAtVersion`: Complete module identifier with version
- `modulePath`: Module path without version
- `version`: Semantic version
- `files`: Array of file objects
  - `path`: Relative path from repository root
  - `isGo`: Whether file is a Go source file

---

### 2. Analyze Package

Analyze a specific Go package within a repository and return all symbols, files, and cross-references for that package.

**Endpoint:** `GET /package/{moduleAtVersion}/{packagePath}`

**Parameters:**
- `moduleAtVersion` (path): URL-encoded module name with version
- `packagePath` (path): Package path relative to repository root (e.g., `runtime`, `cmd/golua-repl`). Optional - omit for root package.

**Example Requests:**
```bash
# Analyze a specific package
curl "http://localhost:8080/api/package/github.com%2Farnodel%2Fgolua%40v0.1.0/runtime"

# Analyze root package
curl "http://localhost:8080/api/package/github.com%2Farnodel%2Fgolua%40v0.1.0"
```

**Response:**
```json
{
  "name": "runtime",
  "path": "/var/folders/.../gonav-cache/github.com_arnodel_golua_v0.1.0",
  "symbols": {
    "New": {
      "name": "New",
      "type": "func",
      "file": "runtime/runtime.go",
      "line": 45,
      "package": "runtime"
    }
  },
  "files": [
    {
      "path": "runtime/runtime.go",
      "isGo": true
    },
    {
      "path": "runtime/arith.go", 
      "isGo": true
    },
    {
      "path": "runtime/table.go",
      "isGo": true
    }
  ]
}
```

**Response Fields:**
- `name`: Package name
- `path`: Absolute path to repository on server
- `symbols`: Map of all symbols defined in this package (public + private). Exported symbols have names starting with uppercase letters.
- `files`: Array of file objects in this package, each with `path` and `isGo` fields. Use `/file/` endpoint to get detailed analysis of individual files.

---

### 3. Get File Content

Retrieve parsed Go source file with symbol information and cross-references.

**Endpoint:** `GET /file/{moduleAtVersion}/{filePath}`

**Parameters:**
- `moduleAtVersion` (path): URL-encoded module name with version
- `filePath` (path): File path relative to repository root (e.g., `cmd/main.go`)

**Example Request:**
```bash
curl "http://localhost:8080/api/file/github.com%2Farnodel%2Fgolua%40v0.1.0/cmd/golua-repl/luabuffer.go"
```

**Response:**
```json
{
  "source": "package main\n\nimport (\n\t\"fmt\"\n)...",
  "references": [
    {
      "name": "fmt",
      "file": "cmd/golua-repl/luabuffer.go", 
      "line": 4,
      "column": 2,
      "target": {
        "name": "fmt",
        "type": "external",
        "file": "",
        "line": 0,
        "package": "fmt",
        "isStdLib": true
      }
    },
    {
      "name": "b", 
      "file": "cmd/golua-repl/luabuffer.go",
      "line": 43,
      "column": 2,
      "target": {
        "name": "b",
        "type": "var",
        "file": "cmd/golua-repl/luabuffer.go", 
        "line": 42,
        "package": "main"
      }
    }
  ]
}
```

**Response Fields:**
- `source`: Raw source code content
- `references`: Array of all symbol references in the file
  - `name`: Referenced symbol name
  - `file`: File containing the reference
  - `line`: Line number of reference
  - `column`: Column position of reference
  - `target`: Symbol being referenced
    - `name`: Target symbol name
    - `type`: Target type (`var`, `func`, `internal`, `external`, etc.)
    - `file`: File containing target (empty for external/unresolved)
    - `line`: Line number of target (0 for external/unresolved)
    - `package`: Package containing target
    - `isStdLib`: True if target is Go standard library
    - `isExternal`: True if target is from different repository
    - `importPath`: Import path for external references
    - `version`: Version for external references

---

## Reference Types

The API distinguishes between several types of symbol references:

### Local References
- **Type**: `var`, `func`, `const`, `type`
- **Scope**: Same file/package
- **Navigation**: Direct file:line navigation
- **Example**: Local variables, same-package functions

### Internal References  
- **Type**: `internal`
- **Scope**: Same repository, different package
- **Navigation**: Cross-package navigation within repository
- **Example**: `runtime.New` within `github.com/arnodel/golua`

### External References
- **Type**: `external` 
- **Scope**: Different repository
- **Navigation**: Cross-repository (requires loading external repo)
- **Example**: `github.com/gin-gonic/gin` functions

### Standard Library References
- **Type**: `external`
- **Flags**: `isStdLib: true`
- **Scope**: Go standard library
- **Navigation**: Not supported (no source available)
- **Example**: `fmt.Println`, `http.Get`

### Builtin References
- **Type**: Various
- **Package**: `builtin`
- **Scope**: Go language builtins
- **Navigation**: Not supported
- **Example**: `int`, `string`, `make`, `len`

---

## Error Responses

All endpoints return appropriate HTTP status codes:

- `200 OK`: Success
- `400 Bad Request`: Invalid module format or file path
- `404 Not Found`: Repository or file not found
- `500 Internal Server Error`: Analysis or processing error

Error responses include a plain text error message in the response body.

---

## Usage Notes

1. **Module Format**: Always use `owner/repo@version` format with proper URL encoding
2. **Caching**: Repositories are cached locally in `/tmp/gonav-cache/`
3. **Cross-References**: The API performs full AST analysis with type checking
4. **Performance**: Initial repository load may take time; subsequent requests are fast
5. **File Types**: Only `.go` files are analyzed; other files return basic content

---

## Frontend Navigation Flow

1. User loads repository → `GET /repo/{module}`
2. User selects file → `GET /file/{module}/{path}` 
3. User clicks symbol → Frontend uses `references` array for navigation:
   - **Local**: Direct navigation to `target.file:target.line`
   - **Internal**: Cross-package navigation using `GET /package/{module}/{package}` to find symbols
   - **External**: Load external repo with `GET /repo/{external-module}`, then navigate
   - **Standard Library/Builtin**: Show informational message

## Package vs File Analysis

- **`/package/`**: Analyzes entire Go package (directory), returns package-level symbols and file list
- **`/file/`**: Analyzes single Go file, returns detailed symbols and references for that file

**Usage Pattern:**
1. Use `/package/` for cross-package navigation and symbol lookup
2. Use `/file/` for displaying individual file content on-demand

This separation keeps package responses lightweight while providing detailed file analysis when needed.