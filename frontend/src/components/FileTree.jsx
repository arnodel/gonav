import React, { useState, useMemo } from 'react'

function FileTree({ repository, onFileSelect, selectedFile }) {
  const [expandedDirectories, setExpandedDirectories] = useState(new Set([''])) // Root is expanded by default
  const [filter, setFilter] = useState('')

  if (!repository || !repository.files) {
    return (
      <div className="file-tree">
        <p>No files to display</p>
      </div>
    )
  }

  // Build directory tree structure
  const directoryTree = useMemo(() => {
    const tree = {}
    
    repository.files.forEach(file => {
      if (file.path === '.') return // Skip the root entry
      
      const pathParts = file.path.split('/')
      let currentLevel = tree
      
      // Build the nested directory structure
      for (let i = 0; i < pathParts.length - 1; i++) {
        const dirName = pathParts[i]
        if (!currentLevel[dirName]) {
          currentLevel[dirName] = {
            type: 'directory',
            children: {},
            path: pathParts.slice(0, i + 1).join('/')
          }
        }
        currentLevel = currentLevel[dirName].children
      }
      
      // Add the file
      const fileName = pathParts[pathParts.length - 1]
      currentLevel[fileName] = {
        type: 'file',
        path: file.path,
        isGo: file.isGo
      }
    })
    
    return tree
  }, [repository.files])

  // Filter files based on search term
  const filteredTree = useMemo(() => {
    if (!filter.trim()) return directoryTree
    
    const filterTree = (node) => {
      const filtered = {}
      
      Object.entries(node).forEach(([name, item]) => {
        if (item.type === 'file') {
          // Include file if it matches the filter
          if (name.toLowerCase().includes(filter.toLowerCase()) || 
              item.path.toLowerCase().includes(filter.toLowerCase())) {
            filtered[name] = item
          }
        } else if (item.type === 'directory') {
          // Recursively filter directory contents
          const filteredChildren = filterTree(item.children)
          if (Object.keys(filteredChildren).length > 0 || 
              name.toLowerCase().includes(filter.toLowerCase())) {
            filtered[name] = {
              ...item,
              children: filteredChildren
            }
          }
        }
      })
      
      return filtered
    }
    
    return filterTree(directoryTree)
  }, [directoryTree, filter])

  const toggleDirectory = (dirPath) => {
    setExpandedDirectories(prev => {
      const newExpanded = new Set(prev)
      if (newExpanded.has(dirPath)) {
        newExpanded.delete(dirPath)
      } else {
        newExpanded.add(dirPath)
      }
      return newExpanded
    })
  }

  const handleFileClick = (filePath) => {
    onFileSelect(filePath)
  }

  const renderTreeItem = (name, item, level = 0, parentPath = '') => {
    const fullPath = parentPath ? `${parentPath}/${name}` : name
    const isExpanded = expandedDirectories.has(fullPath)
    const indentStyle = { paddingLeft: `${level * 16 + 8}px` }

    if (item.type === 'directory') {
      const hasChildren = Object.keys(item.children).length > 0
      
      return (
        <div key={fullPath}>
          <div
            className="file-tree-directory"
            style={indentStyle}
            onClick={() => toggleDirectory(fullPath)}
          >
            <span className="directory-icon">
              {hasChildren ? (isExpanded ? '📂' : '📁') : '📁'}
            </span>
            <span className="directory-name">{name}</span>
          </div>
          {isExpanded && hasChildren && (
            <div className="directory-contents">
              {Object.entries(item.children)
                .sort(([aName, aItem], [bName, bItem]) => {
                  // Directories first, then files
                  if (aItem.type !== bItem.type) {
                    return aItem.type === 'directory' ? -1 : 1
                  }
                  // Alphabetical within same type
                  return aName.localeCompare(bName)
                })
                .map(([childName, childItem]) => 
                  renderTreeItem(childName, childItem, level + 1, fullPath)
                )}
            </div>
          )}
        </div>
      )
    } else {
      // File
      const fileIcon = item.isGo ? '🐹' : '📄'
      const isSelected = selectedFile === item.path
      
      return (
        <div
          key={item.path}
          className={`file-tree-file ${isSelected ? 'selected' : ''}`}
          style={indentStyle}
          onClick={() => handleFileClick(item.path)}
        >
          <span className="file-icon">{fileIcon}</span>
          <span className="file-name">{name}</span>
        </div>
      )
    }
  }

  return (
    <div className="file-tree">
      <div className="file-tree-header">
        <h3>Files</h3>
        <input
          type="text"
          placeholder="Filter files..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="file-filter"
        />
      </div>
      <div className="file-tree-content">
        {Object.entries(filteredTree)
          .sort(([aName, aItem], [bName, bItem]) => {
            // Directories first, then files
            if (aItem.type !== bItem.type) {
              return aItem.type === 'directory' ? -1 : 1
            }
            // Alphabetical within same type
            return aName.localeCompare(bName)
          })
          .map(([name, item]) => renderTreeItem(name, item))
        }
      </div>
    </div>
  )
}

export default FileTree