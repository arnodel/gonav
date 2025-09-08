import React, { useEffect, useRef } from 'react'


function CodeViewer({ file, content, repository, onSymbolClick, onNavigateToSymbol, onDirectNavigateToExternal, highlightLine = null }) {
  const codeRef = useRef(null)

  // Scroll to highlighted line when it changes
  useEffect(() => {
    if (highlightLine && codeRef.current) {
      const lineElement = codeRef.current.querySelector(`[data-line="${highlightLine}"]`)
      if (lineElement) {
        lineElement.scrollIntoView({ 
          behavior: 'smooth', 
          block: 'center' 
        })
      }
    }
  }, [highlightLine, file])

  if (!file || !content) {
    return (
      <div className="code-viewer">
        <div style={{ padding: '2rem', textAlign: 'center', color: '#666' }}>
          Select a file to view its content
        </div>
      </div>
    )
  }

  const handleSymbolClickWithReference = async (reference) => {
    console.log('Symbol clicked via reference:', reference)
    
    if (reference && reference.target) {
      // Check if this is a builtin symbol
      if (reference.target.package === 'builtin') {
        alert(`'${reference.name}' is a Go builtin type/function. Cannot navigate to builtin source.`)
        return
      }
      
      // Check if this is a standard library symbol
      if (reference.target.isStdLib) {
        alert(`'${reference.name}' is a Go standard library symbol from package '${reference.target.package}'. Cannot navigate to standard library source.`)
        return
      }
      
      // Handle external references (cross-repository)
      if (reference.target.isExternal && reference.target.package && reference.target.package.includes('@')) {
        const modulePath = reference.target.importPath || reference.target.package
        const version = reference.target.version || 'latest'
        const moduleAtVersion = `${modulePath}@${version}`
        
        console.log(`Cross-repository reference: ${reference.name} in ${moduleAtVersion}`)
        
        // If we have complete target information (file and line), use it directly
        if (reference.target.file && reference.target.line > 0) {
          console.log(`Using direct navigation: ${reference.target.file}:${reference.target.line}`)
          // Call the navigation function with direct file/line info instead of symbol lookup
          onDirectNavigateToExternal(moduleAtVersion, reference.target.file, reference.target.line, reference.name, reference.line)
          return
        }
        
        // Fallback: Extract package path and use symbol lookup (for cases without complete target info)
        let packagePath = ''
        if (modulePath.includes('/')) {
          const parts = modulePath.split('/')
          if (parts.length > 3) {
            packagePath = parts.slice(3).join('/')
          }
        }
        
        console.log(`Fallback: About to call onNavigateToSymbol with packagePath: '${packagePath}', symbol: '${reference.name}', moduleAtVersion: '${moduleAtVersion}'`)
        onNavigateToSymbol(packagePath, reference.name, moduleAtVersion, reference.line)
        return
      }
      
      
      // Handle internal references that need cross-package resolution
      if (reference.target.type === 'internal' && (!reference.target.file || reference.target.line === 0)) {
        console.log('Internal cross-package reference needs resolution:', reference.name, 'from', reference.target.package)
        // Try to use onNavigateToSymbol for internal cross-package navigation
        if (onNavigateToSymbol && reference.target.package) {
          // Extract package path from the import path (e.g., "github.com/arnodel/golua/runtime" -> "runtime")
          const packageParts = reference.target.package.split('/')
          const packagePath = packageParts[packageParts.length - 1]
          console.log('Attempting internal cross-package navigation to:', packagePath, reference.name)
          onNavigateToSymbol(packagePath, reference.name, null, reference.line)
          return
        }
        alert(`Internal reference to '${reference.name}' from package '${reference.target.package}'. Cross-package navigation needs implementation.`)
        return
      }
      
      // Handle same-repository references (including cross-package within same repo)
      const currentPackagePath = file.includes('/') ? file.substring(0, file.lastIndexOf('/')) : ''
      const targetFile = reference.target.file
      const targetPackagePath = targetFile.includes('/') ? targetFile.substring(0, targetFile.lastIndexOf('/')) : ''
      
      // Cross-package navigation within same repository
      if (targetPackagePath !== currentPackagePath && targetPackagePath && onNavigateToSymbol) {
        console.log('Cross-package navigation to:', targetPackagePath, reference.name)
        onNavigateToSymbol(targetPackagePath, reference.name, null, reference.line)
        return
      }
      
      // Same package navigation (including same file)
      console.log('Same package navigation to:', reference.target.file, 'line:', reference.target.line)
      onSymbolClick(reference.target.file, reference.target.line)
      return
    }
    
    // Fallback: no target information
    console.log('No target information for reference:', reference)
  }

  const handleSymbolClick = async (symbol, clickLine = null, clickColumn = null) => {
    console.log('Symbol clicked:', symbol, 'at line:', clickLine, 'column:', clickColumn)
    
    // Check if using new scope-aware API format
    if (content.definitions && content.scopes) {
      console.log('Using new scope-aware API format')
      return handleNewApiSymbolClick(symbol, clickLine, clickColumn)
    } else {
      console.log('Using legacy API format')
      return handleLegacyApiSymbolClick(symbol, clickLine, clickColumn)
    }
  }

  const handleNewApiSymbolClick = async (symbol, clickLine, clickColumn) => {
    // Use position-based lookup in references array
    if (content.references && clickLine && clickColumn) {
      // Find the reference at the exact click position
      const reference = content.references.find(ref => 
        ref.name === symbol && 
        ref.line === clickLine && 
        Math.abs(ref.column - clickColumn) <= symbol.length  // Allow some tolerance for click position
      )
      
      if (reference) {
        console.log('Found reference:', reference)
        
        // Handle new scope-aware reference types
        if (reference.type === 'local') {
          // Local reference - find definition by definitionId
          const definition = content.definitions.find(def => def.id === reference.definitionId)
          if (definition) {
            console.log('Local reference - navigating to definition:', definition)
            onSymbolClick(file, definition.line) // Navigate within same file
            return
          } else {
            console.log('Definition not found for local reference:', reference.definitionId)
          }
        } else if (reference.target?.isExternal && reference.target?.package && reference.target.package.includes('@')) {
          // External reference (cross-repository) - check this BEFORE internal type check
          const modulePath = reference.target.importPath || reference.target.package
          const version = reference.target.version || 'latest'
          const moduleAtVersion = `${modulePath}@${version}`
          
          console.log(`External cross-repository reference: ${symbol} in ${moduleAtVersion}`)
          
          // Extract package path from the import path
          let packagePath = ''
          if (modulePath.includes('/')) {
            const parts = modulePath.split('/')
            if (parts.length > 3) {
              packagePath = parts.slice(3).join('/')
            }
          }
          
          console.log(`Cross-repository navigation to: packagePath='${packagePath}', symbol='${symbol}', moduleAtVersion='${moduleAtVersion}'`)
          onNavigateToSymbol(packagePath, symbol, moduleAtVersion, clickLine)
          return
        } else if (reference.type === 'internal') {
          // Internal reference - use existing cross-package navigation with reference.target
          if (reference.target && reference.target.file && reference.target.line > 0) {
            console.log('Internal reference with resolved target:', reference.target)
            
            // Check if it's cross-package navigation
            const currentPackagePath = file.includes('/') ? file.substring(0, file.lastIndexOf('/')) : ''
            const targetFile = reference.target.file
            const targetPackagePath = targetFile.includes('/') ? targetFile.substring(0, targetFile.lastIndexOf('/')) : ''
            
            if (targetPackagePath !== currentPackagePath && targetPackagePath && onNavigateToSymbol) {
              console.log('Internal cross-package navigation to:', targetPackagePath, symbol)
              onNavigateToSymbol(targetPackagePath, symbol, null, clickLine)
              return
            }
            
            // Same package navigation
            console.log('Internal same-package navigation to:', reference.target.file, 'line:', reference.target.line)
            onSymbolClick(reference.target.file, reference.target.line)
            return
          } else {
            // Internal reference that needs resolution
            console.log('Internal reference needs resolution:', symbol, 'from', reference.target?.package)
            if (onNavigateToSymbol && reference.target?.package) {
              const packageParts = reference.target.package.split('/')
              const packagePath = packageParts[packageParts.length - 1]
              console.log('Attempting internal cross-package navigation to:', packagePath, symbol)
              onNavigateToSymbol(packagePath, symbol, null, clickLine)
              return
            }
          }
        } else if (reference.target?.isExternal) {
          // External reference - handle cross-repository navigation
          if (reference.target?.isStdLib) {
            alert(`'${symbol}' is a Go standard library symbol from package '${reference.target.package}'. Cannot navigate to standard library source.`)
            return
          }
          
          if (reference.target?.package === 'builtin') {
            alert(`'${symbol}' is a Go builtin type/function. Cannot navigate to builtin source.`)
            return
          }
          
          if (reference.target?.isExternal) {
            const modulePath = reference.target.importPath || reference.target.package
            const version = reference.target.version || 'latest'
            const moduleAtVersion = `${modulePath}@${version}`
            
            console.log(`External cross-repository reference: ${symbol} in ${moduleAtVersion}`)
            
            // Extract package path from the import path
            let packagePath = ''
            if (modulePath.includes('/')) {
              const parts = modulePath.split('/')
              if (parts.length > 3) {
                packagePath = parts.slice(3).join('/')
              }
            }
            
            console.log(`Cross-repository navigation to: packagePath='${packagePath}', symbol='${symbol}', moduleAtVersion='${moduleAtVersion}'`)
            onNavigateToSymbol(packagePath, symbol, moduleAtVersion, clickLine)
            return
          }
        }
        
        // Fallback to legacy handling if reference has target
        if (reference.target) {
          return handleLegacyReference(reference, symbol, clickLine)
        }
      }
    }
    
    // No reference found
    console.log('No reference found at position for symbol:', symbol)
  }

  const handleLegacyApiSymbolClick = async (symbol, clickLine, clickColumn) => {
    // Use position-based lookup in references array instead of name-based lookup in symbols
    if (content.references && clickLine && clickColumn) {
      // Find the reference at the exact click position
      const reference = content.references.find(ref => 
        ref.name === symbol && 
        ref.line === clickLine && 
        Math.abs(ref.column - clickColumn) <= symbol.length  // Allow some tolerance for click position
      )
      
      if (reference && reference.target) {
        return handleLegacyReference(reference, symbol, clickLine)
      }
    }
    
    // Fallback: if no position-based reference found, log debug info
    console.log('No reference found at position for symbol:', symbol)
    console.log('Available references:', content.references?.length || 0)
    if (content.references) {
      const sameNameRefs = content.references.filter(ref => ref.name === symbol)
      console.log(`Found ${sameNameRefs.length} references with same name:`, sameNameRefs.map(ref => `${ref.line}:${ref.column}`))
    }
  }

  const handleLegacyReference = (reference, symbol, clickLine) => {
    console.log('Found reference with target:', reference.target)
    
    // Check if this is a builtin symbol
    if (reference.target.package === 'builtin') {
      alert(`'${symbol}' is a Go builtin type/function. Cannot navigate to builtin source.`)
      return
    }
    
    // Check if this is a standard library symbol
    if (reference.target.isStdLib) {
      alert(`'${symbol}' is a Go standard library symbol from package '${reference.target.package}'. Cannot navigate to standard library source.`)
      return
    }
    
    // Handle external references (cross-repository)
    if (reference.target.isExternal && reference.target.package && reference.target.package.includes('@')) {
      const modulePath = reference.target.importPath || reference.target.package
      const version = reference.target.version || 'latest'
      const moduleAtVersion = `${modulePath}@${version}`
      
      console.log(`Cross-repository reference: ${symbol} in ${moduleAtVersion}`)
      
      // Extract package path from the import path
      let packagePath = ''
      if (modulePath.includes('/')) {
        const parts = modulePath.split('/')
        if (parts.length > 3) {
          packagePath = parts.slice(3).join('/')
        }
      }
      
      console.log(`About to call onNavigateToSymbol with packagePath: '${packagePath}', symbol: '${symbol}', moduleAtVersion: '${moduleAtVersion}'`)
      onNavigateToSymbol(packagePath, symbol, moduleAtVersion, clickLine)
      return
    }
    
    
    // Handle internal references that need cross-package resolution
    if (reference.target.type === 'internal' && (!reference.target.file || reference.target.line === 0)) {
      console.log('Internal cross-package reference needs resolution:', symbol, 'from', reference.target.package)
      // Try to use onNavigateToSymbol for internal cross-package navigation
      if (onNavigateToSymbol && reference.target.package) {
        // Extract package path from the import path (e.g., "github.com/arnodel/golua/runtime" -> "runtime")
        const packageParts = reference.target.package.split('/')
        const packagePath = packageParts[packageParts.length - 1]
        console.log('Attempting internal cross-package navigation to:', packagePath, symbol)
        onNavigateToSymbol(packagePath, symbol, null, clickLine)
        return
      }
      alert(`Internal reference to '${symbol}' from package '${reference.target.package}'. Cross-package navigation needs implementation.`)
      return
    }
    
    // Handle same-repository references (including cross-package within same repo)
    const currentPackagePath = file.includes('/') ? file.substring(0, file.lastIndexOf('/')) : ''
    const targetFile = reference.target.file
    const targetPackagePath = targetFile.includes('/') ? targetFile.substring(0, targetFile.lastIndexOf('/')) : ''
    
    // Cross-package navigation within same repository
    if (targetPackagePath !== currentPackagePath && targetPackagePath && onNavigateToSymbol) {
      console.log('Cross-package navigation to:', targetPackagePath, symbol)
      onNavigateToSymbol(targetPackagePath, symbol, null, clickLine)
      return
    }
    
    // Same package navigation (including same file)
    console.log('Same package navigation to:', reference.target.file, 'line:', reference.target.line)
    onSymbolClick(reference.target.file, reference.target.line)
    return
  }

  const renderCodeWithSymbols = (code) => {
    if (!content.references) return code

    const lines = code.split('\n')
    
    // Create a map of line -> column positions for actual references
    const referenceMap = new Map()
    if (content.references) {
      content.references.forEach((ref, index) => {
        const key = `${ref.line}`
        if (!referenceMap.has(key)) {
          referenceMap.set(key, [])
        }
        referenceMap.get(key).push({
          column: ref.column,
          name: ref.name,
          length: ref.name.length,
          originalIndex: index,  // Keep track of the original index
          type: 'reference'
        })
      })
    }
    
    // Create a map of line -> column positions for definitions (scope-aware API)
    const definitionMap = new Map()
    if (content.definitions) {
      content.definitions.forEach(def => {
        const key = `${def.line}`
        if (!definitionMap.has(key)) {
          definitionMap.set(key, [])
        }
        definitionMap.get(key).push({
          column: def.column,
          name: def.name,
          length: def.name.length,
          definitionId: def.id,
          type: 'definition'
        })
      })
    }
    
    return lines.map((line, index) => {
      const lineNumber = index + 1
      let processedLine = line
      
      // Collect all symbols (references and definitions) on this line
      const lineRefs = referenceMap.get(String(lineNumber)) || []
      const lineDefs = definitionMap.get(String(lineNumber)) || []
      const allSymbols = [...lineRefs, ...lineDefs]
      
      if (allSymbols.length > 0) {
        // Sort by column position in reverse order to avoid position shifts during replacement
        const sortedSymbols = allSymbols.sort((a, b) => b.column - a.column)
        
        sortedSymbols.forEach((symbol) => {
          // Column positions are typically 1-based, convert to 0-based for JavaScript
          const startPos = symbol.column - 1
          const endPos = startPos + symbol.length
          
          // Verify the text at this position matches the symbol name
          // Use the original line, not the processed line, for position checking
          const textAtPosition = line.substring(startPos, endPos)
          if (textAtPosition === symbol.name) {
            // Since we're processing from right to left, the position should still be valid
            const currentText = processedLine.substring(startPos, endPos)
            if (currentText === symbol.name) {
              const before = processedLine.substring(0, startPos)
              const after = processedLine.substring(endPos)
              
              if (symbol.type === 'reference') {
                // Use the original reference index for references
                processedLine = before + `<span class="symbol symbol-reference" data-ref-index="${symbol.originalIndex}">${symbol.name}</span>` + after
              } else if (symbol.type === 'definition') {
                // Use definition ID for definitions
                processedLine = before + `<span class="symbol symbol-definition" data-def-id="${symbol.definitionId}">${symbol.name}</span>` + after
              }
            }
          }
        })
      }
      
      const isHighlighted = highlightLine === lineNumber
      const lineClass = isHighlighted ? 'code-line highlighted' : 'code-line'
      
      return `<div class="${lineClass}" data-line="${lineNumber}">
        <span class="line-number">${lineNumber}</span>
        <span class="line-content">${processedLine}</span>
      </div>`
    }).join('')
  }

  return (
    <div className="code-viewer">
      <div className="code-viewer-header">
        {repository.moduleAtVersion} / {file}
      </div>
      
      <div className="code-content">
        <div 
          ref={codeRef}
          className="code-container"
          dangerouslySetInnerHTML={{ 
            __html: renderCodeWithSymbols(content.source || '') 
          }}
          onClick={(e) => {
            if (e.target.classList.contains('symbol')) {
              // Handle both references and definitions
              if (e.target.dataset.refIndex !== undefined) {
                // Reference click
                const refIndex = parseInt(e.target.dataset.refIndex)
                if (refIndex >= 0 && content.references && content.references[refIndex]) {
                  const reference = content.references[refIndex]
                  handleSymbolClickWithReference(reference)
                }
              } else if (e.target.dataset.defId !== undefined) {
                // Definition click - for now, just show info about the definition
                const defId = e.target.dataset.defId
                const definition = content.definitions?.find(def => def.id === defId)
                if (definition) {
                  console.log('Definition clicked:', definition)
                  // For now, just show definition information
                  // Later we could implement "find all references" functionality
                  alert(`Definition: ${definition.name} (${definition.type}) in scope ${definition.scopeId}`)
                }
              }
            }
          }}
        />
      </div>
    </div>
  )
}

export default CodeViewer