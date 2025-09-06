# Frontend Navigation Architecture

This document explains how the Go Navigator frontend uses the API endpoints to provide seamless code navigation.

## Overview

The frontend implements a **position-based navigation system** that uses AST analysis and type checking to provide accurate symbol resolution, handling complex scenarios like variable scoping, cross-package references, and external dependencies.

## Navigation Flow

### 1. Repository Loading

**User Action:** Enter module name (e.g., `github.com/gin-gonic/gin@v1.9.1`) and click Load

**Frontend Logic:**
```javascript
// RepositoryLoader.jsx & App.jsx
const response = await fetch(`/api/repo/${encodeURIComponent(moduleInput)}`)
const data = await response.json()
setRepository({...data, moduleAtVersion: moduleInput})
```

**API Call:** `GET /repo/{moduleAtVersion}`

**Result:** 
- Repository metadata and file tree loaded
- File tree displayed in sidebar
- URL updated to reflect current repository

---

### 2. File Viewing

**User Action:** Click on a `.go` file in the file tree

**Frontend Logic:**
```javascript
// App.jsx - handleFileSelect()
const response = await fetch(`/api/file/${encodeURIComponent(moduleAtVersion)}/${filePath}`)
const fileData = await response.json()
setSelectedFile({...fileData, filePath})
```

**API Call:** `GET /file/{moduleAtVersion}/{filePath}`

**Result:**
- Source code displayed with syntax highlighting
- Symbols are made clickable using the `references` array
- Each clickable symbol has a `data-ref-index` attribute pointing to its reference

---

### 3. Symbol Navigation (The Core Logic)

**User Action:** Click on any symbol in the source code

**Frontend Logic:** The `CodeViewer.jsx` component implements sophisticated navigation logic:

#### Step 1: Position-Based Lookup
```javascript
// CodeViewer.jsx - handleSymbolClick()
const reference = content.references.find(ref => 
  ref.name === symbol && 
  ref.line === clickLine && 
  Math.abs(ref.column - clickColumn) <= symbol.length
)
```

**Why Position-Based?** Name-based lookup fails with variable scoping. Multiple symbols can have the same name (local variables, method receivers, etc.). Position ensures we get the exact symbol clicked.

#### Step 2: Reference Classification
The frontend then routes navigation based on the reference type:

##### Local References (`target.type` = "var", "func", "const", "type")
```javascript
// Same file navigation
if (reference.target.file && reference.target.line > 0) {
  onSymbolClick(reference.target.file, reference.target.line)
}
```
- **Scope:** Same file or same package
- **Action:** Direct jump to `target.file:target.line`
- **Example:** Local variable `b` → jumps to its declaration

##### Internal Cross-Package References (`target.type` = "internal")
```javascript
// Handle internal references that need cross-package resolution
if (reference.target.type === 'internal' && (!reference.target.file || reference.target.line === 0)) {
  const packageParts = reference.target.package.split('/')
  const packagePath = packageParts[packageParts.length - 1]
  onNavigateToSymbol(packagePath, symbol, null, clickLine)
}
```
- **Scope:** Different package within same repository
- **Action:** Cross-package navigation using package analysis
- **Example:** `runtime.New` within `github.com/arnodel/golua`

##### External References (`target.type` = "external")
```javascript
// Handle external references (cross-repository)
if (reference.target.type === 'external' && reference.target.isExternal) {
  const modulePath = reference.target.importPath || reference.target.package
  const version = reference.target.version || 'latest'
  const moduleAtVersion = `${modulePath}@${version}`
  onNavigateToSymbol(packagePath, symbol, moduleAtVersion, clickLine)
}
```
- **Scope:** Different repository entirely
- **Action:** Load external repository first, then navigate
- **Example:** Functions from `github.com/gin-gonic/gin`

##### Standard Library References (`target.isStdLib` = true)
```javascript
if (reference.target.isStdLib) {
  alert(`'${symbol}' is a Go standard library symbol. Cannot navigate to source.`)
}
```
- **Scope:** Go standard library
- **Action:** Show informational message
- **Example:** `fmt.Println`, `http.Get`

##### Builtin References (`target.package` = "builtin")
```javascript
if (reference.target.package === 'builtin') {
  alert(`'${symbol}' is a Go builtin type/function. Cannot navigate to source.`)
}
```
- **Scope:** Go language builtins
- **Action:** Show informational message  
- **Example:** `int`, `string`, `make`, `len`

---

### 4. Cross-Package Navigation (Internal)

**Trigger:** Clicking on symbols like `runtime.New` from `cmd/golua-repl`

**Frontend Logic:**
```javascript
// App.jsx - navigateToSymbol()
const packageInfo = await analyzePackage(targetModule, packagePath)
let symbol = packageInfo.symbols?.[symbolName]

if (symbol) {
  // Navigate to the symbol's definition
  await handleFileSelect(symbol.file)
  // Highlight the specific line
  setHighlightLine(symbol.line)
}
```

**API Calls:**
1. `GET /package/{moduleAtVersion}/{packagePath}` - Analyze target package
2. `GET /file/{moduleAtVersion}/{filePath}` - Load the file containing the symbol

**Caching:** Package analysis results are cached to avoid redundant API calls:
```javascript
const packageKey = `${moduleAtVersion}::${packagePath}`
if (packages.has(packageKey)) {
  return packages.get(packageKey)  // Use cached result
}
```

---

### 5. Cross-Repository Navigation (External)

**Trigger:** Clicking on symbols from different repositories (e.g., third-party libraries)

**Frontend Logic:**
```javascript
// App.jsx - navigateToSymbol() 
if (!isSameRepo) {
  // Load external repository first
  const loaded = await loadExternalRepository(moduleAtVersion)
  if (!loaded) {
    alert(`Failed to load external repository: ${moduleAtVersion}`)
    return
  }
}
// Then proceed with normal cross-package navigation
```

**API Calls:**
1. `GET /repo/{externalModuleAtVersion}` - Load external repository
2. `GET /package/{externalModuleAtVersion}/{packagePath}` - Analyze target package
3. `GET /file/{externalModuleAtVersion}/{filePath}` - Load the target file

**Multi-Repository State:** The frontend can have multiple repositories loaded simultaneously for cross-repository navigation.

---

### 6. Breadcrumb Navigation

**Purpose:** Track navigation history and allow users to jump back to previous locations

**Frontend Logic:**
```javascript
// App.jsx - addBreadcrumb()
const breadcrumb = {
  filePath,
  fileName,
  line: lineToHighlight,
  symbol,
  moduleAtVersion: targetModule,
  currentModule: repository?.moduleAtVersion,
  timestamp: Date.now()
}
setBreadcrumbs(prev => [...prev, breadcrumb])
```

**Features:**
- **Multi-Repository Breadcrumbs:** Track jumps across different repositories
- **Deep Linking:** Generate shareable URLs for any navigation state
- **History Management:** Forward/back navigation like a web browser

---

## Key Frontend Components

### CodeViewer.jsx
- **Responsibility:** Renders source code with clickable symbols
- **Key Features:**
  - Position-based symbol lookup
  - Reference classification and routing
  - Line highlighting
  - Context menus for symbols

### App.jsx  
- **Responsibility:** Core navigation logic and state management
- **Key Features:**
  - Repository loading and caching
  - Cross-package navigation
  - Breadcrumb management
  - URL routing

### RepositoryLoader.jsx
- **Responsibility:** Repository loading UI
- **Key Features:**
  - Module input validation
  - Loading states
  - Error handling

### FileTree.jsx
- **Responsibility:** File system navigation
- **Key Features:**
  - Hierarchical file display
  - Go file filtering
  - File selection

---

## Caching Strategy

The frontend implements multi-level caching for performance:

### Repository Cache
```javascript
// Repositories are cached by the backend automatically
// Frontend tracks loaded repositories to avoid duplicate API calls
```

### Package Cache
```javascript
const packages = new Map() // packageKey -> PackageInfo
const packageKey = `${moduleAtVersion}::${packagePath}`
packages.set(packageKey, packageInfo)
```

### File Cache
```javascript
// Files are cached implicitly through React state
// selectedFile state persists until user navigates away
```

---

## Error Handling

### Network Errors
- **API timeouts:** Show retry button
- **Connection issues:** Graceful degradation with cached data
- **Server errors:** Clear error messages

### Navigation Errors  
- **Symbol not found:** Alert with helpful message
- **Package analysis failed:** Fallback to basic file view
- **External repository unavailable:** Inform user of limitation

### Input Validation
- **Invalid module format:** Real-time validation in input field
- **Unsupported file types:** Show appropriate message
- **Missing files:** Handle 404s gracefully

---

## Performance Optimizations

1. **Lazy Loading:** Files are only loaded when clicked
2. **Package Caching:** Avoid re-analyzing the same packages
3. **Debounced Navigation:** Prevent rapid-fire navigation calls
4. **Streamlined API Responses:** Removed redundant fields (`imports` and `references` from package endpoint, `symbols` from file endpoint) to reduce payload sizes
5. **Virtual Scrolling:** Handle large files efficiently (future enhancement)
6. **Worker Threads:** Syntax highlighting in background (future enhancement)

---

## Future Enhancements

1. **Semantic Search:** Find symbols across entire repositories
2. **Call Graph Visualization:** Show function call relationships  
3. **Dependency Analysis:** Visualize module dependencies
4. **Offline Mode:** Cache repositories for offline browsing
5. **IDE Integration:** Browser extension for GitHub/GitLab
6. **Performance Profiling:** Show hot paths and bottlenecks