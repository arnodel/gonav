import React, { useState } from 'react'

function RepositoryLoader({ onRepositoryLoad, currentModule = '' }) {
  const [moduleInput, setModuleInput] = useState(currentModule)
  const [loading, setLoading] = useState(false)

  // Update input when currentModule changes (from URL loading)
  React.useEffect(() => {
    setModuleInput(currentModule)
  }, [currentModule])

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!moduleInput.trim()) return

    setLoading(true)
    try {
      const response = await fetch(`http://localhost:8080/api/repo/${encodeURIComponent(moduleInput)}`)
      const data = await response.json()
      
      if (response.ok) {
        onRepositoryLoad({
          ...data,
          moduleAtVersion: moduleInput
        })
      } else {
        alert(`Error loading repository: ${data.error}`)
      }
    } catch (error) {
      console.error('Failed to load repository:', error)
      alert('Failed to load repository')
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="repository-loader">
      <input
        type="text"
        value={moduleInput}
        onChange={(e) => setModuleInput(e.target.value)}
        placeholder="github.com/gin-gonic/gin@v1.9.1"
        className="repository-input"
        disabled={loading}
      />
      <button 
        type="submit" 
        className="load-button"
        disabled={loading || !moduleInput.trim()}
      >
        {loading ? 'Loading...' : 'Load'}
      </button>
    </form>
  )
}

export default RepositoryLoader