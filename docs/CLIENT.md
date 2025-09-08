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
- **Definitions and references** are both made clickable using enhanced arrays
- **Scope-aware highlighting:** Definitions vs references styled differently
- **Enhanced data structure:**
  - `scopes`: Hierarchical code blocks (functions, if statements, loops)
  - `definitions`: Local symbols defined in this file with scope information
  - `references`: Symbol usages with explicit type classification
- Each clickable symbol has attributes linking to the appropriate data structure

---

### 3. Symbol Navigation (The Core Logic)

**User Action:** Click on any symbol in the source code

**Frontend Logic:** The `CodeViewer.jsx` component implements sophisticated scope-aware navigation logic:

#### Step 1: API Format Detection
```javascript
// CodeViewer.jsx - handleSymbolClick()
if (content.definitions && content.scopes) {
  return handleNewApiSymbolClick(symbol, clickLine, clickColumn)
} else {
  return handleLegacyApiSymbolClick(symbol, clickLine, clickColumn)
}
```

**Enhanced API Detection:** The frontend detects whether it's receiving the new scope-aware API format by checking for the presence of `definitions` and `scopes` arrays.

#### Step 2: Position-Based Lookup
```javascript
// CodeViewer.jsx - handleNewApiSymbolClick()
const reference = content.references.find(ref => 
  ref.name === symbol && 
  ref.line === clickLine && 
  Math.abs(ref.column - clickColumn) <= symbol.length
)
```

**Why Position-Based?** Name-based lookup fails with variable scoping. Multiple symbols can have the same name (local variables, method receivers, etc.). Position ensures we get the exact symbol clicked.

#### Step 3: Enhanced Reference Classification
The frontend routes navigation based on the explicit reference type from the enhanced API:

##### Local References (`reference.type` = "local")
```javascript
// Find definition in same file using definitionId
if (reference.type === 'local') {
  const definition = content.definitions.find(def => def.id === reference.definitionId)
  if (definition) {
    onSymbolClick(file, definition.line) // Navigate within same file
    return
  }
}
```
- **Scope:** Same file - locally defined symbols
- **Action:** Direct jump to definition using `definitionId` linkage
- **Examples:** 
  - Local variables: `dumpAtEnd := false`
  - Function parameters: `func main(args []string)`
  - Local functions: `func helper() { ... }`

##### Internal Cross-Package References (`reference.type` = "internal")
```javascript
// Cross-package navigation within same repository
if (reference.type === 'internal' && reference.target.file && reference.target.line > 0) {
  const targetFile = reference.target.file      // "runtime/runtime.go"
  const targetLine = reference.target.line      // 45
  // Check if cross-package navigation needed
  if (targetPackagePath !== currentPackagePath) {
    onNavigateToSymbol(targetPackagePath, symbol, null, clickLine)
  } else {
    onSymbolClick(reference.target.file, reference.target.line)
  }
}
```
- **Scope:** Same repository, different package
- **Action:** Cross-package navigation or direct file navigation
- **Examples:** 
  - `runtime.New` within `github.com/arnodel/golua`
  - `ast.Walk` from local `go/ast` package

##### External References (`reference.type` = "external")
```javascript
// Handle external references (cross-repository or stdlib)
if (reference.target?.isExternal && reference.target?.package?.includes('@')) {
  const modulePath = reference.target.importPath || reference.target.package
  const version = reference.target.version || 'latest'
  const moduleAtVersion = `${modulePath}@${version}`
  onNavigateToSymbol(packagePath, symbol, moduleAtVersion, clickLine)
} else if (reference.target?.isStdLib) {
  alert(`'${symbol}' is a Go standard library symbol from '${reference.target.package}'.`)
}
```
- **Scope:** Different repository or Go standard library
- **Action:** Cross-repository navigation or informational message
- **Examples:** 
  - Third-party: `github.com/gin-gonic/gin@v1.9.1` functions
  - Standard library: `fmt.Println`, `http.Get`
  - Builtins: `make`, `len`, `int`, `string`

#### Step 4: Scope-Aware Features

##### Definition Highlighting
```javascript
// Build definition map for highlighting
const definitionMap = new Map()
if (content.definitions) {
  content.definitions.forEach(def => {
    const key = `${def.line}`
    definitionMap.get(key).push({
      column: def.column,
      name: def.name,
      definitionId: def.id,
      type: 'definition'
    })
  })
}
```

##### Enhanced Symbol Rendering
- **Definitions** are highlighted differently from references
- **Scope information** is available for advanced features
- **Type signatures** are displayed in tooltips

---

### 4. Scope-Aware Features (New)

**The enhanced API enables powerful scope-aware navigation features:**

#### Definition vs Reference Distinction
```javascript
// Different styling for definitions vs references
.symbol-definition {
  background-color: #e3f2fd;  /* Light blue for definitions */
  border-bottom: 2px solid #2196f3;
}

.symbol-reference {
  background-color: #f3e5f5;  /* Light purple for references */
  border-bottom: 1px solid #9c27b0;
}
```

#### Smart Navigation Patterns
- **Click on definition** → Find all references to that symbol
- **Click on reference** → Jump to its definition (local) or source (external)
- **Hover over symbol** → Show type signature and scope information

#### Scope Hierarchy Understanding
```javascript
// Scope utilities for advanced features
function getScopeAtPosition(line, column, scopes) {
  return scopes.find(scope => {
    const { start, end } = scope.range
    return (line >= start.line && line <= end.line) &&
           (column >= start.column && column <= end.column)
  })
}

// Find all symbols visible at a given position
function getSymbolsInScope(line, column, definitions, scopes) {
  const currentScope = getScopeAtPosition(line, column, scopes)
  return definitions.filter(def => 
    isSymbolVisibleInScope(def.scopeId, currentScope?.id || '/')
  )
}
```

#### Advanced Features Enabled
- **Variable shadowing detection**: Identify when local variables shadow outer scope
- **Scope-aware autocomplete**: Only suggest symbols visible at cursor position  
- **Smart refactoring**: Rename variables with proper scope awareness
- **Enhanced debugging**: Show all variables in scope at any position

---

### 5. Cross-Package Navigation (Internal)

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

## Implemented Enhancements ✅

The following advanced features have been successfully implemented:

### ✅ Scope-Aware Navigation
- **Local symbol resolution**: Direct `definitionId` → definition linkage
- **Definition highlighting**: Visual distinction between definitions and references
- **Hierarchical scopes**: Function, block, and method scope analysis
- **Cross-repository navigation**: Full module@version support with isolated environments

### ✅ Enhanced Analysis
- **golang.org/x/tools/go/packages integration**: Accurate type information
- **Isolated environments**: Prevents dependency conflicts
- **Standard library detection**: Proper classification and handling
- **Cross-module reference resolution**: External dependency navigation

### ✅ Performance Optimizations  
- **Position-based lookup**: Eliminates ambiguity in symbol resolution
- **Multi-level caching**: Repository, package, and file-level caching
- **Streamlined API responses**: Reduced payload sizes with targeted data

---

## Future Enhancements 🚧

### Planned Features
1. **Advanced Scope Features:**
   - Variable shadowing detection and warnings
   - Scope-aware autocomplete suggestions
   - Smart refactoring with scope awareness
   
2. **Enhanced Visualization:**
   - Call graph visualization showing function relationships
   - Dependency analysis with interactive module graphs
   - Code coverage visualization overlay

3. **Search & Discovery:**
   - Semantic search across entire repositories
   - Symbol usage analytics and hot path detection
   - Cross-repository symbol search

4. **Integration & Tooling:**
   - IDE integration with VSCode/IntelliJ plugins
   - Browser extensions for GitHub/GitLab code browsing
   - API for third-party tool integration

5. **Performance & Scaling:**
   - Virtual scrolling for large files
   - Background syntax highlighting with Web Workers
   - Incremental analysis for real-time updates
   - Offline mode with local repository caching

### Backward Compatibility

The frontend maintains full backward compatibility:
```javascript
// Automatic API format detection
if (fileContent.definitions && fileContent.scopes) {
  // Use enhanced scope-aware features
  handleEnhancedNavigation(fileContent)
} else {
  // Graceful fallback to legacy behavior  
  handleLegacyNavigation(fileContent)
}
```

---

## Progressive Enhancement Support

The frontend now supports **revision-based progressive enhancement** for faster initial load times and continuously improving analysis quality.

### Progressive Loading Pattern

```javascript
// App.jsx - Enhanced file loading with progressive enhancement
async function loadFileWithEnhancement(filePath) {
  // 1. Get immediate partial results
  const response = await fetch(`/api/file/${moduleAtVersion}/${filePath}`)
  const fileData = await response.json()
  
  // 2. Display partial results immediately
  setSelectedFile({...fileData, filePath})
  
  // 3. Show enhancement indicator if incomplete
  if (!fileData.complete) {
    setEnhancementStatus('loading')
    
    // 4. Poll for improvements
    const pollForEnhancement = async () => {
      const enhanced = await fetch(`/api/file/${moduleAtVersion}/${filePath}?revision=${fileData.revision}`)
      const enhancedData = await enhanced.json()
      
      if (!enhancedData.no_change) {
        // Show improved results
        setSelectedFile({...enhancedData, filePath})
        setEnhancementStatus(enhancedData.complete ? 'complete' : 'partial')
        
        if (!enhancedData.complete) {
          setTimeout(pollForEnhancement, 3000) // Continue polling
        }
      } else {
        setTimeout(pollForEnhancement, 3000) // Retry later
      }
    }
    
    setTimeout(pollForEnhancement, 2000) // Start polling after 2s
  }
}
```

### UI Enhancement Indicators

```javascript
// Component to show progressive enhancement status
function EnhancementStatus({ complete, onRefresh }) {
  if (complete) return null
  
  return (
    <div className="enhancement-banner">
      <span>⏳ Analysis improving... Dependencies loading in background.</span>
      <button onClick={onRefresh}>Check for improvements</button>
    </div>
  )
}
```

For detailed implementation patterns and principles, see [PROGRESSIVE_ENHANCEMENT.md](PROGRESSIVE_ENHANCEMENT.md).