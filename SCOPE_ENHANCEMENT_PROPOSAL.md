# Scope-Aware File API Enhancement Proposal

## Overview

This proposal enhances the `/file/` endpoint to provide scope-aware local definitions, enabling advanced navigation features like definition highlighting, scope-aware symbol suggestions, and better understanding of variable shadowing.

## Enhanced File API Endpoint

### Endpoint
`GET /file/{moduleAtVersion}/{filePath}`

### Example Request
```bash
curl "http://localhost:8080/api/file/github.com%2Farnodel%2Fgolua%40v0.1.0/cmd/golua-repl/main.go"
```

### Response
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
    },
    {
      "id": "/usage",
      "type": "function",
      "name": "usage",
      "range": {
        "start": {"line": 59, "column": 6},
        "end": {"line": 65, "column": 1}
      }
    }
  ],
  
  "definitions": [
    {
      "id": "def_1",
      "name": "defaultInitLua",
      "type": "var", 
      "line": 3,
      "column": 5,
      "scopeId": "/",
      "signature": "[]byte"
    },
    {
      "id": "def_2",
      "name": "dumpAtEnd",
      "type": "var",
      "line": 6,
      "column": 2,
      "scopeId": "/main",
      "signature": "bool"
    },
    {
      "id": "def_3", 
      "name": "err",
      "type": "var",
      "line": 7,
      "column": 2,
      "scopeId": "/main",
      "signature": "error"
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
      "name": "err", 
      "line": 26,
      "column": 9,
      "type": "local",
      "definitionId": "def_3"
    },
    {
      "name": "fmt",
      "line": 4,
      "column": 2,
      "type": "external",
      "target": {
        "package": "fmt",
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
        "package": "runtime"
      }
    }
  ]
}
```

### Response Fields

#### Root Fields
- `source`: Raw source code content
- `scopes`: Array of scope objects (excluding implicit global scope "/")
- `definitions`: Array of local symbol definitions
- `references`: Array of all symbol references in the file

#### Scope Object
- `id`: Hierarchical scope identifier using "/" as separator
  - Global scope has implicit ID "/"
  - Function scopes: "/functionName" 
  - Block scopes: "/parent/block_N" (numbered within parent)
  - Method scopes: "/ReceiverType_methodName"
- `type`: Scope type ("function", "block", "type", etc.)
- `name`: Human-readable name (for functions, methods, types)
- `range`: Position range where scope is valid
  - `start`: {"line": N, "column": N}
  - `end`: {"line": N, "column": N}

#### Definition Object  
- `id`: Unique definition identifier within file
- `name`: Symbol name
- `type`: Symbol type ("var", "const", "func", "type", "param")
- `line`: Line number where defined
- `column`: Column position where defined
- `scopeId`: ID of scope containing this definition ("/" for global)
- `signature`: Type signature string

#### Reference Object
- `name`: Referenced symbol name  
- `line`: Line number of reference
- `column`: Column position of reference
- `type`: Reference type with three possible values:
  - `"local"`: Reference to local definition in same file
  - `"internal"`: Reference to symbol in same repository, different package  
  - `"external"`: Reference to symbol in different repository or standard library

For local references:
- `definitionId`: ID of local definition being referenced

For internal references:
- `target`: Object with cross-package symbol information:
  - `name`: Symbol name
  - `type`: Symbol type ("function", "var", "const", "type")
  - `file`: Relative file path where symbol is defined
  - `line`: Line number of definition
  - `column`: Column position of definition
  - `package`: Package name containing the symbol
  - `signature`: Type signature (optional)

For external references:
- `target`: Object with external symbol information:
  - `name`: Symbol name
  - `type`: Always "external"
  - `file`: Empty string (not available)
  - `line`: 0 (not available)
  - `column`: 0 (not available)
  - `package`: Package name or import path
  - `importPath`: Full import path (for external packages)
  - `isStdLib`: Boolean indicating if this is Go standard library
  - `isExternal`: Boolean indicating if this is from different repository
  - `version`: Version string (for external dependencies, optional)

## Backend Implementation Plan

### Phase 1: Extend Analyzer Core (internal/analyzer/analyzer.go)

1. **Update FileInfo struct** to include new fields:
   ```go
   type FileInfo struct {
       Path        string              `json:"path"`
       Source      string              `json:"source"`
       Scopes      []*ScopeInfo        `json:"scopes"`
       Definitions []*Definition       `json:"definitions"`
       References  []*Reference        `json:"references"`
   }
   ```

2. **Add new data structures**:
   ```go
   type ScopeInfo struct {
       ID    string    `json:"id"`
       Type  string    `json:"type"`
       Name  string    `json:"name,omitempty"`
       Range Range     `json:"range"`
   }
   
   type Definition struct {
       ID        string `json:"id"`
       Name      string `json:"name"`
       Type      string `json:"type"`
       Line      int    `json:"line"`
       Column    int    `json:"column"`
       ScopeID   string `json:"scopeId"`
       Signature string `json:"signature"`
   }
   
   type Range struct {
       Start Position `json:"start"`
       End   Position `json:"end"`
   }
   
   type Position struct {
       Line   int `json:"line"`
       Column int `json:"column"`
   }
   ```

3. **Enhance analyzeFile method**:
   - Extract scope information from `types.Info.Scopes`
   - Build hierarchical scope IDs using meaningful names
   - Separate local definitions from cross-package references
   - Map references to definition IDs for local symbols

### Phase 2: Scope Extraction Logic

1. **Implement scope traversal**:
   ```go
   func (a *PackageAnalyzer) extractScopes(file *ast.File, info *types.Info) []*ScopeInfo {
       // Walk types.Info.Scopes to build scope hierarchy
       // Generate hierarchical IDs: "/", "/main", "/main/if_1", etc.
       // Extract position ranges from AST nodes
   }
   ```

2. **Implement definition extraction**:
   ```go
   func (a *PackageAnalyzer) extractDefinitions(file *ast.File, info *types.Info, scopes []*ScopeInfo) []*Definition {
       // Walk info.Defs to find local symbol definitions
       // Determine which scope each definition belongs to
       // Generate unique definition IDs
   }
   ```

3. **Update reference classification**:
   ```go
   func (a *PackageAnalyzer) classifyReference(ref *Reference, localDefs []*Definition) {
       // Classify as local/internal/external
       // For local: set definitionId
       // For internal/external: populate target object
   }
   ```

### Phase 3: Update File Endpoint (main.go)

1. **Remove symbol conversion logic**: The new FileInfo structure directly matches the API response
2. **Update response mapping**: Map analyzer.FileInfo directly to JSON response
3. **Maintain backward compatibility**: Ensure existing clients can ignore new fields

## Frontend Implementation Plan

### Immediate Goal: Restore Current Functionality

The primary objective is to update the frontend to work with the new API structure while maintaining all existing navigation features.

### Phase 1: Update Reference Handling (CodeViewer.jsx)

1. **Modify handleSymbolClick method** to handle new reference types:
   ```javascript
   // Current logic looks for reference.target
   // New logic needs to handle three cases:
   
   if (reference.type === 'local') {
     // Find definition by definitionId
     const definition = content.definitions.find(def => def.id === reference.definitionId)
     if (definition) {
       // Navigate to definition.line, definition.column
       onSymbolClick(currentFile, definition.line)
     }
   } else if (reference.type === 'internal') {
     // Use existing cross-package navigation with reference.target
     // This should work exactly as before since target structure is same
     const targetFile = reference.target.file
     const targetLine = reference.target.line
     onSymbolClick(targetFile, targetLine)
   } else if (reference.type === 'external') {
     // Handle external references (same as current external handling)
     // Show appropriate message or attempt external navigation
   }
   ```

2. **Add backward compatibility**:
   ```javascript
   // Check if using new API format
   if (content.definitions && content.scopes) {
     // Use new scope-aware logic
     handleNewApiFormat(reference)
   } else {
     // Fall back to current logic for old API responses
     handleOldApiFormat(reference)
   }
   ```

3. **Update symbol highlighting logic**:
   ```javascript
   // Current: Makes all references clickable
   // New: Also make definitions clickable and highlight differently
   
   // Build reference map (existing logic)
   const referenceMap = new Map()
   content.references.forEach((ref, index) => { /* existing code */ })
   
   // Build definition map (new logic)
   const definitionMap = new Map()
   if (content.definitions) {
     content.definitions.forEach(def => {
       const key = `${def.line}`
       if (!definitionMap.has(key)) {
         definitionMap.set(key, [])
       }
       definitionMap.get(key).push(def)
     })
   }
   ```

### Phase 2: Minimal Definition Highlighting

1. **Add CSS classes for definitions**:
   ```css
   .symbol-definition {
     background-color: #e3f2fd;  /* Light blue background */
     border-bottom: 2px solid #2196f3;
   }
   
   .symbol-reference {
     background-color: #f3e5f5;  /* Light purple background */
     border-bottom: 1px solid #9c27b0;
   }
   ```

2. **Update renderCodeWithSymbols to mark definitions**:
   ```javascript
   // Existing logic marks references as clickable
   // Add similar logic to mark definitions
   
   const lineRefs = referenceMap.get(String(lineNumber))
   const lineDefs = definitionMap.get(String(lineNumber))
   
   // Process both references and definitions when highlighting line
   ```

### Phase 3: Utility Functions (JavaScript)

Create helper functions to work with the new data structure:

```javascript
// utils/scopeUtils.js

// Check if scope A is ancestor of scope B
export function isAncestorScope(ancestorId, descendantId) {
  if (ancestorId === '/') return true  // Global scope is ancestor of all
  if (descendantId === '/') return false  // Nothing is ancestor of global
  return descendantId.startsWith(ancestorId + '/')
}

// Find scope containing a position
export function getScopeAtPosition(line, column, scopes) {
  return scopes.find(scope => {
    const { start, end } = scope.range
    return (line > start.line || (line === start.line && column >= start.column)) &&
           (line < end.line || (line === end.line && column <= end.column))
  })
}

// Get all definitions visible at a position
export function getSymbolsInScope(line, column, definitions, scopes) {
  const currentScope = getScopeAtPosition(line, column, scopes)
  const currentScopeId = currentScope ? currentScope.id : '/'
  
  return definitions.filter(def => {
    return def.scopeId === '/' ||  // Global symbols always visible
           def.scopeId === currentScopeId ||  // Same scope
           isAncestorScope(def.scopeId, currentScopeId)  // Parent scope
  })
}
```

### Compatibility Strategy

1. **Graceful degradation**: If new fields aren't present, fall back to current behavior
2. **Progressive enhancement**: Add new features only when new API data is available  
3. **No breaking changes**: All existing functionality must continue working

### Testing Approach

1. **Test with old API**: Ensure existing functionality works unchanged
2. **Test with new API**: Verify new definition highlighting works
3. **Test mixed scenarios**: Handle cases where some files use old API, others use new

## Migration Strategy

### Backward Compatibility
- **Frontend**: Check for presence of new fields, fallback to old behavior
- **API**: All new fields are additions, no breaking changes
- **Gradual adoption**: Clients can adopt new features incrementally

### Rollout Plan
1. **Phase 1**: Backend implementation with feature flag
2. **Phase 2**: Frontend support for new fields (with fallbacks) 
3. **Phase 3**: Enable new features in UI
4. **Phase 4**: Advanced scope-aware features

## Benefits Enabled

### Immediate Benefits
- **Definition highlighting**: Click variable → highlight its definition
- **Clearer navigation**: "Go to definition" vs "Find all references"
- **Better UX**: Distinguish local variables from package symbols

### Future Benefits  
- **Scope-aware autocomplete**: Only suggest symbols in scope
- **Smart refactoring**: Rename with scope awareness
- **Variable shadowing detection**: Warn when variables shadow outer scope
- **Advanced debugging**: Show all variables in scope at breakpoint

## Testing Strategy

### Backend Tests
- Unit tests for scope extraction from various AST patterns
- Integration tests comparing old vs new API responses
- Performance tests to ensure no regression in response times

### Frontend Tests  
- Component tests for definition highlighting
- Integration tests for scope-aware navigation
- Backward compatibility tests with old API responses

This enhancement maintains full backward compatibility while enabling powerful new navigation features that make the code browser significantly more useful for understanding complex codebases.