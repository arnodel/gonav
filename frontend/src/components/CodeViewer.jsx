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
    
    // First check if it's a defined symbol in this file
    if (content.symbols && content.symbols[symbol]) {
      const symbolInfo = content.symbols[symbol]
      console.log('Found symbol definition:', symbolInfo)
      
      if (symbolInfo.file === file) {
        // Same file - scroll to line and highlight
        console.log(`Navigate to line ${symbolInfo.line} in current file`)
        onSymbolClick(symbolInfo.file, symbolInfo.line)
      } else {
        // Different file - load it with line to highlight
        console.log('Navigating to different file:', symbolInfo.file, 'line:', symbolInfo.line)
        onSymbolClick(symbolInfo.file, symbolInfo.line)
      }
      return
    }
    
    // Check if it's a reference that points to a definition elsewhere
    if (content.references) {
      const reference = content.references.find(ref => ref.name === symbol)
      if (reference && reference.target) {
        console.log('Found reference with target:', reference.target)
        
        // Check if target is in a different package (cross-package navigation)
        if (reference.target.package && onNavigateToSymbol) {
          console.log('Cross-package navigation to:', reference.target.package, symbol)
          onNavigateToSymbol(reference.target.package, symbol)
          return
        }
        
        // Same package navigation
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
          const textAtPosition = line.substring(startPos, endPos)
          if (textAtPosition === ref.name) {
            const before = line.substring(0, startPos)
            const after = line.substring(endPos)
            processedLine = before + `<span class="symbol" data-symbol="${ref.name}">${ref.name}</span>` + after
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