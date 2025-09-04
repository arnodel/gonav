import React, { useEffect, useRef } from 'react'

function CodeViewer({ file, content, repository, onSymbolClick, highlightLine = null }) {
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
        onSymbolClick(reference.target.file, reference.target.line)
        return
      }
    }
    
    console.log('No symbol info found for:', symbol)
    console.log('Available symbols:', Object.keys(content.symbols || {}))
    console.log('Available references:', content.references?.length || 0)
  }

  const renderCodeWithSymbols = (code) => {
    if (!content.symbols) return code

    const lines = code.split('\n')
    
    return lines.map((line, index) => {
      const lineNumber = index + 1
      let processedLine = line
      
      // Make symbols clickable
      Object.keys(content.symbols).forEach(symbol => {
        const regex = new RegExp(`\\b${symbol}\\b`, 'g')
        processedLine = processedLine.replace(regex, `<span class="symbol" data-symbol="${symbol}">${symbol}</span>`)
      })
      
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