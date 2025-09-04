import React, { useState, useEffect } from 'react'
import RepositoryLoader from './components/RepositoryLoader'
import FileTree from './components/FileTree'
import CodeViewer from './components/CodeViewer'

function App() {
  const [repository, setRepository] = useState(null)
  const [currentFile, setCurrentFile] = useState(null)
  const [fileContent, setFileContent] = useState(null)
  const [highlightLine, setHighlightLine] = useState(null)

  // URL routing functions
  const updateURL = (moduleAtVersion, filePath = null) => {
    const encodedModule = encodeURIComponent(moduleAtVersion)
    let url = `/repo/${encodedModule}`
    if (filePath) {
      const encodedPath = encodeURIComponent(filePath)
      url += `/file/${encodedPath}`
    }
    window.history.pushState({ moduleAtVersion, filePath }, '', url)
  }

  const parseURL = () => {
    const path = window.location.pathname
    const repoMatch = path.match(/^\/repo\/([^\/]+)/)
    
    if (repoMatch) {
      const moduleAtVersion = decodeURIComponent(repoMatch[1])
      const fileMatch = path.match(/^\/repo\/[^\/]+\/file\/(.+)$/)
      const filePath = fileMatch ? decodeURIComponent(fileMatch[1]) : null
      
      return { moduleAtVersion, filePath }
    }
    
    return { moduleAtVersion: null, filePath: null }
  }

  // Load state from URL on page load
  useEffect(() => {
    const { moduleAtVersion, filePath } = parseURL()
    if (moduleAtVersion) {
      loadRepositoryFromURL(moduleAtVersion, filePath)
    }

    // Handle browser back/forward navigation
    const handlePopState = (event) => {
      const { moduleAtVersion, filePath } = parseURL()
      if (moduleAtVersion) {
        loadRepositoryFromURL(moduleAtVersion, filePath)
      } else {
        setRepository(null)
        setCurrentFile(null)
        setFileContent(null)
      }
    }

    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  const loadRepositoryFromURL = async (moduleAtVersion, filePath) => {
    try {
      const response = await fetch(`http://localhost:8080/api/repo/${encodeURIComponent(moduleAtVersion)}`)
      const data = await response.json()
      
      if (response.ok) {
        const repo = {
          ...data,
          moduleAtVersion: moduleAtVersion
        }
        setRepository(repo)
        
        if (filePath) {
          // Load the specific file
          const fileResponse = await fetch(`http://localhost:8080/api/file/${encodeURIComponent(moduleAtVersion)}/${filePath}`)
          const fileData = await fileResponse.json()
          setCurrentFile(filePath)
          setFileContent(fileData)
        }
      }
    } catch (error) {
      console.error('Failed to load from URL:', error)
    }
  }

  const handleRepositoryLoad = (repo) => {
    setRepository(repo)
    setCurrentFile(null)
    setFileContent(null)
    setHighlightLine(null)
    updateURL(repo.moduleAtVersion)
  }

  const handleFileSelect = async (filePath, lineToHighlight = null, updateURLFlag = true) => {
    if (!repository) return
    
    try {
      const response = await fetch(`http://localhost:8080/api/file/${encodeURIComponent(repository.moduleAtVersion)}/${filePath}`)
      const data = await response.json()
      setCurrentFile(filePath)
      setFileContent(data)
      setHighlightLine(lineToHighlight)
      
      if (updateURLFlag) {
        updateURL(repository.moduleAtVersion, filePath)
      }
    } catch (error) {
      console.error('Failed to load file:', error)
    }
  }

  return (
    <div className="app">
      <header className="app-header">
        <h1>Go Navigator</h1>
        <RepositoryLoader 
          onRepositoryLoad={handleRepositoryLoad} 
          currentModule={repository?.moduleAtVersion || ''} 
        />
      </header>
      
      <div className="app-content">
        {repository && (
          <>
            <aside className="sidebar">
              <FileTree 
                repository={repository} 
                onFileSelect={handleFileSelect}
                selectedFile={currentFile}
              />
            </aside>
            
            <main className="main-content">
              <CodeViewer 
                file={currentFile}
                content={fileContent}
                repository={repository}
                onSymbolClick={handleFileSelect}
                highlightLine={highlightLine}
              />
            </main>
          </>
        )}
      </div>
    </div>
  )
}

export default App