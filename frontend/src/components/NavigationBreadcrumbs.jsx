import React from 'react'

function NavigationBreadcrumbs({ breadcrumbs, currentIndex, onNavigate, onClear }) {
  if (!breadcrumbs || breadcrumbs.length === 0) {
    return null
  }

  return (
    <div className="navigation-breadcrumbs">
      <div className="breadcrumbs-header">
        <h4>Navigation History</h4>
        <button 
          type="button" 
          className="breadcrumbs-clear"
          onClick={onClear}
          title="Clear history"
        >
          ✕
        </button>
      </div>
      <div className="breadcrumbs-list">
        {breadcrumbs.map((breadcrumb, index) => {
          const isCurrent = index === currentIndex
          const isFuture = currentIndex !== -1 && index < currentIndex
          
          return (
            <div 
              key={index} 
              className={`breadcrumb-item ${isCurrent ? 'current' : ''} ${isFuture ? 'future' : ''}`}
              onClick={() => onNavigate(index)}
              title={`${breadcrumb.moduleAtVersion}/${breadcrumb.filePath}${breadcrumb.line ? `:${breadcrumb.line}` : ''}`}
            >
            <span className="breadcrumb-number">{index + 1}.</span>
            <div className="breadcrumb-content">
              <div className="breadcrumb-location">
                <span className="breadcrumb-file">{breadcrumb.fileName}</span>
                {breadcrumb.symbol && (
                  <span className="breadcrumb-symbol">→ {breadcrumb.symbol}</span>
                )}
              </div>
              {breadcrumb.moduleAtVersion !== breadcrumb.currentModule && (
                <div className="breadcrumb-module">{breadcrumb.moduleAtVersion}</div>
              )}
            </div>
          </div>
          )
        })}
      </div>
    </div>
  )
}

export default NavigationBreadcrumbs