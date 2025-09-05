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

  const handleSymbolClick = async (symbol) => {
    console.log('Symbol clicked:', symbol)
    
    // First check if it's a defined symbol in this file (but skip external references)
    if (content.symbols && content.symbols[symbol]) {
      const symbolInfo = content.symbols[symbol]
      console.log('Found symbol definition:', symbolInfo)
      
      // Skip external references - they should be handled by the reference logic below
      if (symbolInfo.type === 'external') {
        console.log('Skipping external symbol, will handle via references')
      } else if (symbolInfo.file === file) {
        // Same file - scroll to line and highlight
        console.log(`Navigate to line ${symbolInfo.line} in current file`)
        onSymbolClick(symbolInfo.file, symbolInfo.line)
        return
      } else {
        // Different file - load it with line to highlight
        console.log('Navigating to different file:', symbolInfo.file, 'line:', symbolInfo.line)
        onSymbolClick(symbolInfo.file, symbolInfo.line)
        return
      }
    }
    
    // Check if it's a reference that points to a definition elsewhere
    if (content.references) {
      const reference = content.references.find(ref => ref.name === symbol)
      if (reference && reference.target) {
        console.log('Found reference with target:', reference.target)
        
        // Check if this is an external reference (lazy resolution)
        if (reference.target.type === 'external') {
          console.log('External reference detected:', reference.target)
          
          // Check if this is a cross-repository reference
          if (reference.target.isExternal) {
            // Cross-repository reference - use the enhanced navigation
            const modulePath = reference.target.importPath || reference.target.package
            const version = reference.target.version || 'latest'
            const moduleAtVersion = `${modulePath}@${version}`
            
            console.log(`Cross-repository reference: ${symbol} in ${moduleAtVersion}`)
            
            // Extract package path from the import path
            let packagePath = ''
            if (modulePath.includes('/')) {
              const parts = modulePath.split('/')
              // For github.com/owner/repo -> '' (root package)
              // For github.com/owner/repo/subpackage -> 'subpackage'
              if (parts.length > 3) {
                packagePath = parts.slice(3).join('/')
              }
            }
            
            console.log(`About to call onNavigateToSymbol with packagePath: '${packagePath}', symbol: '${symbol}', moduleAtVersion: '${moduleAtVersion}'`)
            
            // Use the enhanced navigation function
            onNavigateToSymbol(packagePath, symbol, moduleAtVersion)
            console.log(`onNavigateToSymbol call completed`)
            return
          }
          
          // Same-repository external reference - extract package path
          const importPath = reference.target.package
          let packagePath = importPath
          
          // If it's from the same module, extract the relative path
          if (importPath.includes('/')) {
            const parts = importPath.split('/')
            // Look for common module patterns and extract the package path
            // For github.com/owner/repo/path/to/package -> path/to/package
            const moduleStart = parts.findIndex((part, index) => 
              index >= 2 && !part.includes('.') // Skip domain and owner parts
            )
            if (moduleStart > 0 && moduleStart < parts.length - 1) {
              packagePath = parts.slice(moduleStart + 1).join('/')
            } else {
              // Fallback: use the last part as package name
              packagePath = parts[parts.length - 1]
            }
          }
          
          console.log('Resolved same-repo package path:', packagePath)
          onNavigateToSymbol(packagePath, symbol)
          return
        }
        
        // Determine current package from the file path
        const currentPackagePath = file.includes('/') ? file.substring(0, file.lastIndexOf('/')) : ''
        
        // Check if target is in a different package (cross-package navigation)
        const targetFile = reference.target.file
        const targetPackagePath = targetFile.includes('/') ? targetFile.substring(0, targetFile.lastIndexOf('/')) : ''
        
        // Cross-package navigation if target is in a different package within the same repo
        if (targetPackagePath !== currentPackagePath && targetPackagePath && onNavigateToSymbol) {
          console.log('Cross-package navigation to:', targetPackagePath, symbol)
          onNavigateToSymbol(targetPackagePath, symbol)
          return
        }
        
        // Same package navigation (including same file)
        onSymbolClick(reference.target.file, reference.target.line)
        return
      }
    }
    
    console.log('No symbol info found for:', symbol)
    console.log('Available symbols:', Object.keys(content.symbols || {}))
    console.log('Available references:', content.references?.length || 0)
  }

  const renderCodeWithSymbols = (code) => {
    if (!content.symbols && !content.references) return code

    const lines = code.split('\n')
    
    // Create a map of line -> column positions for actual references
    const referenceMap = new Map()
    if (content.references) {
      content.references.forEach(ref => {
        const key = `${ref.line}`
        if (!referenceMap.has(key)) {
          referenceMap.set(key, [])
        }
        referenceMap.get(key).push({
          column: ref.column,
          name: ref.name,
          length: ref.name.length
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
        
        sortedRefs.forEach(ref => {
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
              processedLine = before + `<span class="symbol" data-symbol="${ref.name}">${ref.name}</span>` + after
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
              handleSymbolClick(e.target.dataset.symbol)
            }
          }}
        />
      </div>
    </div>
  )
}

export default CodeViewer