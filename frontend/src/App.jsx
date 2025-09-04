import React, { useState, useEffect } from 'react'
import RepositoryLoader from './components/RepositoryLoader'
import FileTree from './components/FileTree'
import CodeViewer from './components/CodeViewer'

function App() {
  const [repository, setRepository] = useState(null)
  const [currentFile, setCurrentFile] = useState(null)
  const [fileContent, setFileContent] = useState(null)
  const [highlightLine, setHighlightLine] = useState(null)
  const [packages, setPackages] = useState(new Map()) // packagePath -> packageInfo cache

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

  const analyzePackage = async (moduleAtVersion, packagePath) => {
    const packageKey = `${moduleAtVersion}::${packagePath}`
    
    // Check if package is already analyzed
    if (packages.has(packageKey)) {
      console.log(`Using cached package: ${packagePath}`)
      return packages.get(packageKey)
    }

    try {
      console.log(`Analyzing package: ${packagePath}`)
      const encodedModule = encodeURIComponent(moduleAtVersion)
      const url = packagePath 
        ? `http://localhost:8080/api/package/${encodedModule}/${packagePath}`
        : `http://localhost:8080/api/package/${encodedModule}`
      
      const response = await fetch(url)
      const packageInfo = await response.json()
      
      if (response.ok) {
        // Cache the analyzed package
        setPackages(prev => new Map(prev).set(packageKey, packageInfo))
        console.log(`Cached package: ${packagePath} with ${Object.keys(packageInfo.symbols || {}).length} symbols`)
        return packageInfo
      } else {
        console.error('Failed to analyze package:', packageInfo.error)
        return null
      }
    } catch (error) {
      console.error('Error analyzing package:', error)
      return null
    }
  }

  const handleRepositoryLoad = (repo) => {
    setRepository(repo)
    setCurrentFile(null)
    setFileContent(null)
    setHighlightLine(null)
    setPackages(new Map()) // Clear package cache for new repository
    updateURL(repo.moduleAtVersion)
  }

  const handleFileSelect = async (filePath, lineToHighlight = null, updateURLFlag = true) => {
    if (!repository) return
    
    try {
      console.log(`Loading file: ${filePath}`)
      
      // The backend will automatically analyze the package containing this file
      const response = await fetch(`http://localhost:8080/api/file/${encodeURIComponent(repository.moduleAtVersion)}/${filePath}`)
      const data = await response.json()
      
      if (response.ok) {
        setCurrentFile(filePath)
        setFileContent(data)
        setHighlightLine(lineToHighlight)
        
        if (updateURLFlag) {
          updateURL(repository.moduleAtVersion, filePath)
        }
      } else {
        console.error('Failed to load file:', data.error)
        alert(`Failed to load file: ${data.error}`)
      }
    } catch (error) {
      console.error('Failed to load file:', error)
      alert('Failed to load file')
    }
  }

  // New function for cross-package navigation
  const navigateToSymbol = async (packagePath, symbolName, moduleAtVersion = null) => {
    const targetModule = moduleAtVersion || repository?.moduleAtVersion
    if (!targetModule) return

    console.log(`Navigating to symbol: ${symbolName} in package: ${packagePath}`)

    try {
      // Analyze the target package
      const packageInfo = await analyzePackage(targetModule, packagePath)
      if (!packageInfo) {
        alert(`Failed to analyze package: ${packagePath}`)
        return
      }

      // Find the symbol in the package
      const symbol = packageInfo.symbols?.[symbolName]
      if (symbol) {
        console.log(`Found symbol: ${symbolName} at ${symbol.file}:${symbol.line}`)
        
        // Navigate to the file containing the symbol
        await handleFileSelect(symbol.file, symbol.line)
      } else {
        console.log(`Symbol ${symbolName} not found in package ${packagePath}`)
        alert(`Symbol '${symbolName}' not found in package '${packagePath}'`)
      }
    } catch (error) {
      console.error('Error navigating to symbol:', error)
      alert('Failed to navigate to symbol')
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
                onNavigateToSymbol={navigateToSymbol}
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