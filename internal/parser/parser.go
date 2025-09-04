package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"strings"
)

type GoParser struct {
	fileSet *token.FileSet
}

type FileContent struct {
	Source  string             `json:"source"`
	Symbols map[string]Symbol  `json:"symbols"`
}

type Symbol struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "function", "type", "var", "const"
	File     string `json:"file"`
	Line     int    `json:"line"`
	Package  string `json:"package,omitempty"`
}

func New() *GoParser {
	return &GoParser{
		fileSet: token.NewFileSet(),
	}
}

func (p *GoParser) ParseFile(absolutePath, relativePath string) (*FileContent, error) {
	// Read source file
	sourceBytes, err := ioutil.ReadFile(absolutePath)
	if err != nil {
		return nil, err
	}

	source := string(sourceBytes)

	// Only parse Go files for AST analysis
	if !strings.HasSuffix(absolutePath, ".go") {
		return &FileContent{
			Source:  source,
			Symbols: make(map[string]Symbol),
		}, nil
	}

	// Parse the Go file
	file, err := parser.ParseFile(p.fileSet, absolutePath, sourceBytes, parser.ParseComments)
	if err != nil {
		// If parsing fails, still return the source
		return &FileContent{
			Source:  source,
			Symbols: make(map[string]Symbol),
		}, nil
	}

	// Extract symbols from AST
	symbols := p.extractSymbols(file, relativePath)

	return &FileContent{
		Source:  source,
		Symbols: symbols,
	}, nil
}

func (p *GoParser) extractSymbols(file *ast.File, relativePath string) map[string]Symbol {
	symbols := make(map[string]Symbol)
	fmt.Printf("Extracting symbols from file: '%s'\n", relativePath)

	// Walk the AST and extract symbols
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Name != nil {
				pos := p.fileSet.Position(node.Pos())
				symbol := Symbol{
					Name: node.Name.Name,
					Type: "function",
					File: relativePath,
					Line: pos.Line,
				}
				symbols[node.Name.Name] = symbol
				fmt.Printf("Found function: %s in file: %s\n", node.Name.Name, relativePath)
			}

		case *ast.GenDecl:
			for _, spec := range node.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name != nil {
						pos := p.fileSet.Position(s.Pos())
						symbol := Symbol{
							Name: s.Name.Name,
							Type: "type",
							File: relativePath,
							Line: pos.Line,
						}
						symbols[s.Name.Name] = symbol
						fmt.Printf("Found type: %s in file: %s\n", s.Name.Name, relativePath)
					}

				case *ast.ValueSpec:
					for _, name := range s.Names {
						if name != nil {
							pos := p.fileSet.Position(name.Pos())
							symbolType := "var"
							if node.Tok == token.CONST {
								symbolType = "const"
							}
							symbols[name.Name] = Symbol{
								Name: name.Name,
								Type: symbolType,
								File: relativePath,
								Line: pos.Line,
							}
						}
					}
				}
			}

		case *ast.InterfaceType:
			// Extract interface methods
			if node.Methods != nil {
				for _, method := range node.Methods.List {
					for _, name := range method.Names {
						if name != nil {
							pos := p.fileSet.Position(name.Pos())
							symbols[name.Name] = Symbol{
								Name: name.Name,
								Type: "method",
								File: relativePath,
								Line: pos.Line,
							}
						}
					}
				}
			}

		case *ast.StructType:
			// Extract struct fields
			if node.Fields != nil {
				for _, field := range node.Fields.List {
					for _, name := range field.Names {
						if name != nil {
							pos := p.fileSet.Position(name.Pos())
							symbols[name.Name] = Symbol{
								Name: name.Name,
								Type: "field",
								File: relativePath,
								Line: pos.Line,
							}
						}
					}
				}
			}
		}

		return true
	})

	return symbols
}