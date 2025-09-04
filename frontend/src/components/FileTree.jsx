import React from 'react'

function FileTree({ repository, onFileSelect, selectedFile }) {
  if (!repository || !repository.files) {
    return (
      <div className="file-tree">
        <p>No files to display</p>
      </div>
    )
  }

  const handleFileClick = (filePath) => {
    onFileSelect(filePath)
  }

  return (
    <div className="file-tree">
      <h3>Files</h3>
      {repository.files.map((file) => (
        <div
          key={file.path}
          className={`file-tree-item ${selectedFile === file.path ? 'selected' : ''}`}
          onClick={() => handleFileClick(file.path)}
        >
          {file.path}
        </div>
      ))}
    </div>
  )
}

export default FileTree