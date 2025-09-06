import React, { useEffect, useRef } from 'react'


function CodeViewer({ file, content, repository, onSymbolClick, onNavigateToSymbol, highlightLine = null }) {
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
      if (reference.target.type === 'external' && reference.target.isExternal) {
        const modulePath = reference.target.importPath || reference.target.package
        const version = reference.target.version || 'latest'
        const moduleAtVersion = `${modulePath}@${version}`
        
        console.log(`Cross-repository reference: ${reference.name} in ${moduleAtVersion}`)
        
        // Extract package path from the import path
        let packagePath = ''
        if (modulePath.includes('/')) {
          const parts = modulePath.split('/')
          if (parts.length > 3) {
            packagePath = parts.slice(3).join('/')
          }
        }
        
        console.log(`About to call onNavigateToSymbol with packagePath: '${packagePath}', symbol: '${reference.name}', moduleAtVersion: '${moduleAtVersion}'`)
        onNavigateToSymbol(packagePath, reference.name, moduleAtVersion, reference.line)
        return
      }
      
      // Handle external references (cross-repository) that need to be resolved first
      if (reference.target.type === 'external' && (!reference.target.file || reference.target.line === 0)) {
        console.log('External reference needs resolution:', reference.name, 'from', reference.target.importPath)
        alert(`External reference to '${reference.name}' from ${reference.target.importPath || reference.target.package}. Cross-repository navigation not yet implemented.`)
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
    
    // Use position-based lookup in references array instead of name-based lookup in symbols
    if (content.references && clickLine && clickColumn) {
      // Find the reference at the exact click position
      const reference = content.references.find(ref => 
        ref.name === symbol && 
        ref.line === clickLine && 
        Math.abs(ref.column - clickColumn) <= symbol.length  // Allow some tolerance for click position
      )
      
      if (reference && reference.target) {
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
        if (reference.target.type === 'external' && reference.target.isExternal) {
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
        
        // Handle external references (cross-repository) that need to be resolved first
        if (reference.target.type === 'external' && (!reference.target.file || reference.target.line === 0)) {
          console.log('External reference needs resolution:', symbol, 'from', reference.target.importPath)
          alert(`External reference to '${symbol}' from ${reference.target.importPath || reference.target.package}. Cross-repository navigation not yet implemented.`)
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
    }
    
    // Fallback: if no position-based reference found, log debug info
    console.log('No reference found at position for symbol:', symbol)
    console.log('Available references:', content.references?.length || 0)
    if (content.references) {
      const sameNameRefs = content.references.filter(ref => ref.name === symbol)
      console.log(`Found ${sameNameRefs.length} references with same name:`, sameNameRefs.map(ref => `${ref.line}:${ref.column}`))
    }
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
          originalIndex: index  // Keep track of the original index
        })
      })
    }
    
    return lines.map((line, index) => {
      const lineNumber = index + 1
      let processedLine = line
      
      // Make only actual references clickable (not all symbol occurrences)
      const lineRefs = referenceMap.get(String(lineNumber))
      if (lineRefs) {
        // Sort by column position in reverse order to avoid position shifts during replacement
        const sortedRefs = [...lineRefs].sort((a, b) => b.column - a.column)
        
        sortedRefs.forEach((ref, index) => {
          // Column positions are typically 1-based, convert to 0-based for JavaScript
          const startPos = ref.column - 1
          const endPos = startPos + ref.length
          
          // Verify the text at this position matches the reference name
          // Use the original line, not the processed line, for position checking
          const textAtPosition = line.substring(startPos, endPos)
          if (textAtPosition === ref.name) {
            // For string replacement, we need to work backwards since we sorted in reverse order
            // Find the current position in the processedLine that corresponds to the original position
            let currentStartPos = startPos
            let currentEndPos = endPos
            
            // Since we're processing from right to left, the position should still be valid
            const currentText = processedLine.substring(currentStartPos, currentEndPos)
            if (currentText === ref.name) {
              const before = processedLine.substring(0, currentStartPos)
              const after = processedLine.substring(currentEndPos)
              
              // Use the original reference index we stored
              processedLine = before + `<span class="symbol" data-ref-index="${ref.originalIndex}">${ref.name}</span>` + after
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
              // Get the reference index directly from the DOM element
              const refIndex = parseInt(e.target.dataset.refIndex)
              if (refIndex >= 0 && content.references && content.references[refIndex]) {
                const reference = content.references[refIndex]
                handleSymbolClickWithReference(reference)
              }
            }
          }}
        />
      </div>
    </div>
  )
}

export default CodeViewer