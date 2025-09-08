import React, { useState, useEffect } from 'react'
import RepositoryLoader from './components/RepositoryLoader'
import FileTree from './components/FileTree'
import CodeViewer from './components/CodeViewer'
import NavigationBreadcrumbs from './components/NavigationBreadcrumbs'

function App() {
  const [repository, setRepository] = useState(null)
  const [currentFile, setCurrentFile] = useState(null)
  const [fileContent, setFileContent] = useState(null)
  const [highlightLine, setHighlightLine] = useState(null)
  const [packages, setPackages] = useState(new Map()) // packagePath -> packageInfo cache
  const [breadcrumbs, setBreadcrumbs] = useState([]) // Navigation history
  const [historyIndex, setHistoryIndex] = useState(-1) // Current position in history (-1 = no history)

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

  const addBreadcrumb = (filePath, lineToHighlight, symbol, moduleAtVersion) => {
    const targetModule = moduleAtVersion || repository?.moduleAtVersion
    if (!targetModule || !filePath) return

    const fileName = filePath.split('/').pop()
    const breadcrumb = {
      filePath,
      fileName,
      line: lineToHighlight,
      symbol,
      moduleAtVersion: targetModule,
      currentModule: repository?.moduleAtVersion,
      timestamp: Date.now()
    }

    setBreadcrumbs(prev => {
      // If we're not at the end of history, truncate everything after current position
      const currentHistory = historyIndex === -1 ? prev : prev.slice(historyIndex)
      
      // Add new breadcrumb at the beginning (most recent first)
      const newBreadcrumbs = [breadcrumb, ...currentHistory]
      
      // Keep only last 20 entries
      return newBreadcrumbs.slice(0, 20)
    })
    
    // Reset to beginning of history (most recent)
    setHistoryIndex(0)
  }

  const handleBreadcrumbNavigate = async (breadcrumbIndex) => {
    // Set history index to the clicked position
    setHistoryIndex(breadcrumbIndex)
    
    const breadcrumb = breadcrumbs[breadcrumbIndex]
    if (!breadcrumb) return
    
    if (breadcrumb.moduleAtVersion !== repository?.moduleAtVersion) {
      // Cross-repository navigation - but don't add to history
      await navigateToExternalRepo(breadcrumb)
    } else {
      // Same repository navigation - don't add to history
      await navigateWithoutHistory(breadcrumb.filePath, breadcrumb.line, breadcrumb.moduleAtVersion)
    }
  }

  const navigateWithoutHistory = async (filePath, lineToHighlight = null, moduleAtVersion = null) => {
    const targetModule = moduleAtVersion || repository?.moduleAtVersion
    if (!targetModule) return
    
    try {
      console.log(`Loading file: ${filePath} from ${targetModule}`)
      
      const response = await fetch(`http://localhost:8080/api/file/${encodeURIComponent(targetModule)}/${filePath}`)
      const data = await response.json()
      
      if (response.ok) {
        setCurrentFile(filePath)
        setFileContent(data)
        setHighlightLine(lineToHighlight)
        updateURL(repository.moduleAtVersion, filePath)
        // Note: Don't add breadcrumb here - this is for history navigation
      } else {
        console.error('Failed to load file:', data.error)
        alert(`Failed to load file: ${data.error}`)
      }
    } catch (error) {
      console.error('Failed to load file:', error)
      alert('Failed to load file')
    }
  }

  const navigateToExternalRepo = async (breadcrumb) => {
    // Similar to navigateToSymbol but for history navigation
    console.log(`Navigating to external repository from history: ${breadcrumb.moduleAtVersion}`)
    
    try {
      const response = await fetch(`http://localhost:8080/api/repo/${encodeURIComponent(breadcrumb.moduleAtVersion)}`)
      const repoData = await response.json()
      
      if (response.ok) {
        const repo = {
          ...repoData,
          moduleAtVersion: breadcrumb.moduleAtVersion
        }
        setRepository(repo)
        setCurrentFile(null)
        setFileContent(null)
        setHighlightLine(null)
        setPackages(new Map()) // Clear package cache for new repository
        
        // Navigate to the file in the new repository
        setTimeout(async () => {
          await navigateWithoutHistory(breadcrumb.filePath, breadcrumb.line, breadcrumb.moduleAtVersion)
        }, 100)
      } else {
        alert(`Failed to switch to external repository: ${repoData.error}`)
      }
    } catch (error) {
      console.error('Error navigating to external repo from history:', error)
      alert('Failed to navigate from history')
    }
  }

  const handleFileSelect = async (filePath, lineToHighlight = null, updateURLFlag = true, moduleAtVersion = null, symbol = null, clickLine = null) => {
    const targetModule = moduleAtVersion || repository?.moduleAtVersion
    if (!targetModule) return

    // If we're navigating to a different file, add current location to breadcrumbs
    if (currentFile && currentFile !== filePath && repository?.moduleAtVersion) {
      const currentFileName = currentFile.split('/').pop()
      addBreadcrumb(currentFile, clickLine || highlightLine, symbol ? `clicked on ${symbol}` : `viewed ${currentFileName}`, repository.moduleAtVersion)
    }
    
    try {
      console.log(`Loading file: ${filePath} from ${targetModule}`)
      
      // The backend will automatically analyze the package containing this file
      const response = await fetch(`http://localhost:8080/api/file/${encodeURIComponent(targetModule)}/${filePath}`)
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

  // Load external repository (same as regular repository loading)
  const loadExternalRepository = async (moduleAtVersion) => {
    try {
      console.log(`Loading external repository: ${moduleAtVersion}`)
      const response = await fetch(`http://localhost:8080/api/repo/${encodeURIComponent(moduleAtVersion)}`)
      const data = await response.json()
      
      if (response.ok) {
        console.log(`External repository loaded: ${moduleAtVersion}`)
        return true
      } else {
        console.error('Failed to load external repository:', response.statusText)
        return false
      }
    } catch (error) {
      console.error('Error loading external repository:', error)
      return false
    }
  }

  // New function for cross-package navigation
  const navigateToSymbol = async (packagePath, symbolName, moduleAtVersion = null, clickLine = null) => {
    const targetModule = moduleAtVersion || repository?.moduleAtVersion
    if (!targetModule) return

    console.log(`Navigating to symbol: ${symbolName} in package: ${packagePath}`)

    // Check if target package is in the same repository
    const currentModule = repository?.moduleAtVersion
    const isSameRepo = !moduleAtVersion || moduleAtVersion === currentModule
    
    if (!isSameRepo) {
      // External repository - load it first
      console.log(`Cross-repository navigation to ${moduleAtVersion}/${packagePath}.${symbolName}`)
      
      const loaded = await loadExternalRepository(moduleAtVersion)
      if (!loaded) {
        alert(`Failed to load external repository: ${moduleAtVersion}`)
        return
      }
    }

    try {
      // Analyze the target package (works for both same-repo and external)
      const packageInfo = await analyzePackage(targetModule, packagePath)
      if (!packageInfo) {
        alert(`Failed to analyze package: ${packagePath}`)
        return
      }

      // Find the symbol in all symbols (exported symbols are those starting with uppercase)
      let symbol = packageInfo.symbols?.[symbolName]
      
      if (symbol) {
        console.log(`Found symbol: ${symbolName} at ${symbol.file}:${symbol.line}`)
        
        if (isSameRepo) {
          // Same repository - navigate directly
          await handleFileSelect(symbol.file, symbol.line, true, null, symbolName, clickLine)
        } else {
          // External repository - switch to new repo and navigate
          console.log(`Switching to external repository: ${moduleAtVersion}`)
          
          // Load the external repository as the main repository
          const response = await fetch(`http://localhost:8080/api/repo/${encodeURIComponent(moduleAtVersion)}`)
          const repoData = await response.json()
          
          if (response.ok) {
            const repo = {
              ...repoData,
              moduleAtVersion: moduleAtVersion
            }
            setRepository(repo)
            setCurrentFile(null)
            setFileContent(null)
            setHighlightLine(null)
            setPackages(new Map()) // Clear package cache for new repository
            
            // Navigate to the symbol in the new repository
            setTimeout(async () => {
              await handleFileSelect(symbol.file, symbol.line, true, moduleAtVersion, symbolName, clickLine)
            }, 100) // Small delay to ensure state updates
          } else {
            alert(`Failed to switch to external repository: ${repoData.error}`)
          }
        }
      } else {
        console.log(`Symbol ${symbolName} not found in package ${packagePath}`)
        const availableExported = Object.keys(packageInfo.symbols || {}).filter(name => 
          name.length > 0 && name[0] >= 'A' && name[0] <= 'Z'
        )
        console.log(`Available exported symbols in ${packagePath}:`, availableExported)
        alert(`Symbol '${symbolName}' not found in package '${packagePath}'`)
      }
    } catch (error) {
      console.error('Error navigating to symbol:', error)
      alert('Failed to navigate to symbol')
    }
  }

  // Direct navigation for external references with complete target info
  const onDirectNavigateToExternal = async (moduleAtVersion, targetFile, targetLine, symbolName, clickLine = null) => {
    console.log(`Direct external navigation to ${moduleAtVersion}/${targetFile}:${targetLine}`)
    
    try {
      // Load external repository first
      const loaded = await loadExternalRepository(moduleAtVersion)
      if (!loaded) {
        alert(`Failed to load external repository: ${moduleAtVersion}`)
        return
      }

      // Switch to external repository
      console.log(`Switching to external repository: ${moduleAtVersion}`)
      
      const response = await fetch(`http://localhost:8080/api/repo/${encodeURIComponent(moduleAtVersion)}`)
      const repoData = await response.json()
      
      if (response.ok) {
        const repo = {
          ...repoData,
          moduleAtVersion: moduleAtVersion
        }
        setRepository(repo)
        setCurrentFile(null)
        setFileContent(null)
        setHighlightLine(null)
        setPackages(new Map()) // Clear package cache for new repository
        
        // Navigate directly to the file and line
        setTimeout(async () => {
          await handleFileSelect(targetFile, targetLine, true, moduleAtVersion, symbolName, clickLine)
        }, 100) // Small delay to ensure state updates
      } else {
        alert(`Failed to switch to external repository: ${repoData.error}`)
      }
    } catch (error) {
      console.error('Error in direct external navigation:', error)
      alert('Failed to navigate to external symbol')
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
              <div className="sidebar-top">
                <FileTree 
                  repository={repository} 
                  onFileSelect={handleFileSelect}
                  selectedFile={currentFile}
                />
              </div>
              <div className="sidebar-bottom">
                <NavigationBreadcrumbs 
                  breadcrumbs={breadcrumbs}
                  currentIndex={historyIndex}
                  onNavigate={handleBreadcrumbNavigate}
                  onClear={() => {
                    setBreadcrumbs([])
                    setHistoryIndex(-1)
                  }}
                />
              </div>
            </aside>
            
            <main className="main-content">
              <CodeViewer 
                file={currentFile}
                content={fileContent}
                repository={repository}
                onSymbolClick={handleFileSelect}
                onNavigateToSymbol={navigateToSymbol}
                onDirectNavigateToExternal={onDirectNavigateToExternal}
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