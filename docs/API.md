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
    "Add": {
      "name": "Add",
      "type": "function",
      "file": "runtime/arith.go",
      "line": 24,
      "column": 6,
      "package": "runtime",
      "signature": "func(x runtime.Value, y runtime.Value) (runtime.Value, bool)",
      "isStdLib": true
    },
    "New": {
      "name": "New", 
      "type": "function",
      "file": "runtime/runtime.go",
      "line": 45,
      "column": 6,
      "package": "runtime",
      "signature": "func() *runtime.Runtime",
      "isStdLib": true
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
- `symbols`: Map of symbol names to symbol definitions. Contains both exported (uppercase) and unexported (lowercase) symbols. Key = symbol name, Value = symbol object with:
  - `name`: Symbol identifier
  - `type`: Symbol type (`function`, `var`, `const`, `type`, etc.)
  - `file`: Relative file path where symbol is defined
  - `line`: Line number of definition
  - `column`: Column position of definition  
  - `package`: Package name containing the symbol
  - `signature`: Type signature (for functions, variables, etc.)
  - `isStdLib`: Boolean indicating if this is a standard library symbol
- `files`: Array of file objects in this package:
  - `path`: Relative file path from repository root
  - `isGo`: Boolean indicating if this is a Go source file
  
  Use `/file/` endpoint to get detailed analysis of individual files.

---

### 3. Get File Content (Enhanced with Scope Analysis)

Retrieve parsed Go source file with enhanced scope-aware symbol information, local definitions, and cross-references.

**Endpoint:** `GET /file/{moduleAtVersion}/{filePath}`

**Parameters:**
- `moduleAtVersion` (path): URL-encoded module name with version
- `filePath` (path): File path relative to repository root (e.g., `cmd/main.go`)

**Example Request:**
```bash
curl "http://localhost:8080/api/file/github.com%2Farnodel%2Fgolua%40v0.1.0/cmd/golua-repl/main.go"
```

**Response:**
```json
{
  "source": "package main\n\nvar defaultInitLua []byte\n\nfunc main() {\n\tdumpAtEnd := false\n\terr := someCall()\n\tif err != nil {\n\t\tlog.Fatal(err)\n\t}\n}",
  
  "scopes": [
    {
      "id": "/main",
      "type": "function",
      "name": "main",
      "range": {
        "start": {"line": 5, "column": 6},
        "end": {"line": 11, "column": 1}
      }
    },
    {
      "id": "/main/if_1",
      "type": "block", 
      "range": {
        "start": {"line": 8, "column": 16},
        "end": {"line": 10, "column": 3}
      }
    }
  ],
  
  "definitions": [
    {
      "id": "def_1",
      "name": "defaultInitLua",
      "type": "variable", 
      "line": 3,
      "column": 5,
      "scopeId": "/",
      "signature": "[]byte"
    },
    {
      "id": "def_2",
      "name": "dumpAtEnd",
      "type": "variable",
      "line": 6,
      "column": 2,
      "scopeId": "/main",
      "signature": "bool"
    }
  ],
  
  "references": [
    {
      "name": "dumpAtEnd",
      "line": 20,
      "column": 14,
      "type": "local",
      "definitionId": "def_2"
    },
    {
      "name": "fmt",
      "line": 4,
      "column": 2,
      "type": "external",
      "target": {
        "name": "fmt",
        "type": "type",
        "file": "",
        "line": 0,
        "column": 0,
        "package": "fmt",
        "signature": "package fmt",
        "importPath": "fmt",
        "isExternal": true,
        "isStdLib": true
      }
    },
    {
      "name": "New",
      "line": 45,
      "column": 15,
      "type": "internal", 
      "target": {
        "name": "New",
        "type": "function",
        "file": "runtime/runtime.go", 
        "line": 45,
        "column": 6,
        "package": "runtime",
        "signature": "func() *Runtime",
        "importPath": "github.com/arnodel/golua/runtime",
        "isExternal": false,
        "isStdLib": false
      }
    }
  ]
}
```

**Response Fields:**

#### Root Fields
- `source`: Raw source code content
- `scopes`: Array of scope objects representing code blocks (functions, if statements, loops, etc.)
- `definitions`: Array of local symbol definitions within this file
- `references`: Array of all symbol references in the file

#### Scope Object
Scopes represent code blocks like functions, if statements, for loops, etc.
- `id`: Hierarchical scope identifier using "/" as separator
  - Global scope has implicit ID "/"
  - Function scopes: "/functionName" 
  - Block scopes: "/parent/block_N" (numbered within parent)
  - Method scopes: "/ReceiverType_methodName"
- `type`: Scope type (`"function"`, `"block"`, `"type"`, `"method"`)
- `name`: Human-readable name (for functions, methods, types)
- `range`: Position range where scope is valid
  - `start`: `{"line": N, "column": N}`
  - `end`: `{"line": N, "column": N}`

#### Definition Object  
Local definitions are symbols defined within this file (variables, functions, constants, types, parameters).
- `id`: Unique definition identifier within file (e.g., `"def_1"`, `"def_main"`)
- `name`: Symbol name
- `type`: Symbol type (`"variable"`, `"constant"`, `"function"`, `"type"`, `"parameter"`)
- `line`: Line number where defined
- `column`: Column position where defined
- `scopeId`: ID of scope containing this definition (`"/"` for global scope)
- `signature`: Type signature string

#### Reference Object
References represent all symbol usages in the file.
- `name`: Referenced symbol name  
- `line`: Line number of reference
- `column`: Column position of reference
- `type`: Reference type with three possible values:
  - `"local"`: Reference to local definition in same file
  - `"internal"`: Reference to symbol in same repository, different package  
  - `"external"`: Reference to symbol in different repository or standard library

**For local references:**
- `definitionId`: ID of local definition being referenced (links to `definitions` array)

**For internal references:**
- `target`: Object with cross-package symbol information:
  - `name`: Symbol name
  - `type`: Symbol type (`"function"`, `"variable"`, `"constant"`, `"type"`)
  - `file`: Relative file path where symbol is defined
  - `line`: Line number of definition
  - `column`: Column position of definition
  - `package`: Package name containing the symbol
  - `signature`: Type signature
  - `importPath`: Full import path within repository
  - `isExternal`: `false` (same repository)
  - `isStdLib`: `false` (not standard library)

**For external references:**
- `target`: Object with external symbol information:
  - `name`: Symbol name
  - `type`: Symbol type or `"external"` for unresolved
  - `file`: Empty string (not available for external symbols)
  - `line`: 0 (not available)
  - `column`: 0 (not available)
  - `package`: Package name or module path (may include `@version` for external modules)
  - `signature`: Type signature (when available)
  - `importPath`: Full import path
  - `isExternal`: `true` (different repository)
  - `isStdLib`: `true/false` (indicates if Go standard library)
  - `version`: Version string (for external dependencies)

---

## Reference Types

The enhanced API distinguishes between three main types of symbol references:

### Local References
- **Type**: `"local"`
- **Scope**: Same file - symbols defined within the current file
- **Data**: Contains `definitionId` linking to the `definitions` array
- **Navigation**: Jump to definition within same file using `definitions[].line`
- **Examples**: 
  - Local variables: `dumpAtEnd := false`
  - Function parameters: `func processFile(filename string)`
  - Local function definitions: `func helper() { ... }`

### Internal References  
- **Type**: `"internal"`
- **Scope**: Same repository, different package
- **Data**: Contains `target` object with cross-package symbol information
- **Navigation**: Cross-package navigation within repository
- **Examples**: 
  - `runtime.New` within `github.com/arnodel/golua`
  - `ast.Walk` from `go/ast` package within same repository
- **Target fields**:
  - `file`: Relative path like `"runtime/runtime.go"`
  - `line`/`column`: Exact position in target file
  - `isExternal`: `false`
  - `isStdLib`: `false`

### External References
- **Type**: `"external"` 
- **Scope**: Different repository or Go standard library
- **Data**: Contains `target` object with external symbol information
- **Navigation**: Cross-repository (requires loading external repo) or informational message
- **Examples**: 
  - Third-party modules: `github.com/gin-gonic/gin` functions
  - Standard library: `fmt.Println`, `http.Get`
  - Language builtins: `make`, `len`, `int`, `string`
- **Target fields**:
  - `file`: Empty string `""`
  - `line`/`column`: `0` (not available)
  - `package`: May include `@version` for external modules
  - `isExternal`: `true`
  - `isStdLib`: `true` for standard library, `false` for third-party
  - `version`: Version string (when available for external deps)

### Scope-Aware Features

The new API also provides:

#### Definitions Array
- Lists all symbols **defined** within the file
- Each definition includes its containing scope (`scopeId`)
- Enables "Go to Definition" functionality for local symbols

#### Scopes Array  
- Hierarchical representation of code blocks (functions, if statements, loops, etc.)
- Enables scope-aware features like variable shadowing detection
- Scope IDs use hierarchical naming: `"/"`, `"/main"`, `"/main/if_1"`

#### Enhanced Navigation
- **Local symbols**: `reference.definitionId` → `definitions.find(d => d.id === definitionId)`
- **Cross-package**: `reference.target.file` + `reference.target.line`
- **External**: `reference.target.package` + optional `reference.target.version`

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

### Enhanced Scope-Aware Navigation

1. **User loads repository** → `GET /repo/{module}`
2. **User selects file** → `GET /file/{module}/{path}` 
3. **User clicks symbol** → Frontend uses enhanced `references` array for navigation:

   - **Local References** (`type: "local"`):
     ```javascript
     // Find definition in same file
     const definition = definitions.find(d => d.id === reference.definitionId)
     if (definition) {
       // Jump to definition.line, definition.column
       navigateToLine(definition.line)
     }
     ```

   - **Internal References** (`type: "internal"`):
     ```javascript
     // Cross-package navigation within repository
     const targetFile = reference.target.file      // "runtime/runtime.go"
     const targetLine = reference.target.line      // 45
     // Load and navigate to target file
     loadFile(targetFile).then(() => navigateToLine(targetLine))
     ```

   - **External References** (`type: "external"`):
     ```javascript
     if (reference.target.isStdLib) {
       // Standard library - show informational message
       showMessage(`'${reference.name}' is from Go standard library package '${reference.target.package}'`)
     } else if (reference.target.package.includes('@')) {
       // External module with version - cross-repository navigation
       const [modulePath, version] = reference.target.package.split('@')
       loadExternalRepo(modulePath, version).then(() => navigateToSymbol(reference.name))
     } else {
       // Builtin or unresolved
       showMessage(`Cannot navigate to builtin symbol '${reference.name}'`)
     }
     ```

### Advanced Features Enabled

#### Definition Highlighting
- Highlight all symbol **definitions** using the `definitions` array
- Visually distinguish definitions from references with different styling

#### Scope-Aware Symbol Resolution  
- Use `scopes` array to understand variable shadowing
- Show which scope contains each symbol definition
- Enable "find all references in scope" functionality

#### Smart Navigation
- **Click on definition** → Find all references to that symbol
- **Click on reference** → Jump to definition or external source
- **Hover over symbol** → Show scope information and type signature

## Package vs File Analysis

- **`/package/`**: Analyzes entire Go package (directory), returns package-level symbols and file list
- **`/file/`**: Analyzes single Go file, returns detailed symbols and references for that file

**Usage Pattern:**
1. Use `/package/` for cross-package navigation and symbol lookup
2. Use `/file/` for displaying individual file content on-demand

This separation keeps package responses lightweight while providing detailed file analysis when needed.

---

## API Evolution & Backward Compatibility

### Enhanced File API (Current)

The current `/file/` endpoint returns the enhanced scope-aware format with:
- ✅ `scopes` array for code block analysis
- ✅ `definitions` array for local symbol definitions  
- ✅ `references` array with `type` field (`"local"`, `"internal"`, `"external"`)
- ✅ Enhanced `target` objects with `isExternal`, `isStdLib`, `version` fields

### Backward Compatibility

The API maintains backward compatibility:
- **Frontend detection**: Check for presence of `definitions` and `scopes` fields
- **Graceful fallback**: Older clients can ignore new fields
- **Progressive enhancement**: New features only activate when new data is available

```javascript
// Frontend compatibility pattern
if (fileContent.definitions && fileContent.scopes) {
  // Use enhanced scope-aware navigation
  handleEnhancedApi(fileContent)
} else {
  // Fall back to legacy navigation
  handleLegacyApi(fileContent)
}
```

### Key Enhancements Over Legacy API

| Feature | Legacy API | Enhanced API |
|---------|------------|--------------|
| **Local symbols** | Mixed with cross-package refs | Separate `definitions` array with `definitionId` links |
| **Reference types** | Target-based detection | Explicit `type` field (`local`/`internal`/`external`) |
| **Scope information** | Not available | Hierarchical `scopes` array with ranges |
| **External modules** | Basic `isExternal` flag | Full `package@version` format with `version` field |
| **Standard library** | `isStdLib` flag only | Enhanced detection with proper `importPath` |
| **Definition navigation** | Target-based navigation | Direct `definitionId` → `definitions` lookup |

### Migration Benefits

- **Improved accuracy**: Scope-aware analysis eliminates false positives
- **Better UX**: Clear distinction between definitions and references  
- **Enhanced navigation**: Precise local symbol resolution
- **Future-ready**: Enables advanced features like variable shadowing detection