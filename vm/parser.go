package vm

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// ParseGoFile parses a Go source file into an AST.
func ParseGoFile(path string) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse file '%s': %w", path, err)
	}
	return fset, file, nil
}

// FindFunction locates a top-level function declaration by name.
func FindFunction(file *ast.File, name string) (*ast.FuncDecl, error) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Recv != nil {
			continue
		}
		if fn.Name != nil && fn.Name.Name == name {
			return fn, nil
		}
	}
	return nil, fmt.Errorf("function %s not defined", name)
}

// FunctionStatements returns the body statements of a parsed function.
func FunctionStatements(fn *ast.FuncDecl) ([]ast.Stmt, error) {
	if fn == nil {
		return nil, fmt.Errorf("function body is nil")
	}
	if fn.Body == nil {
		return nil, fmt.Errorf("function body is nil")
	}
	return append([]ast.Stmt(nil), fn.Body.List...), nil
}

// CompileFunctionFile parses a Go file, extracts a named function, and compiles it into block.
func (c *Compiler) CompileFunctionFile(block *Block, path, functionName string) error {
	if c == nil {
		return fmt.Errorf("compiler is nil")
	}
	if block == nil {
		return fmt.Errorf("block is nil")
	}

	_, file, err := ParseGoFile(path)
	if err != nil {
		return err
	}

	fn, err := FindFunction(file, functionName)
	if err != nil {
		return err
	}

	stmts, err := FunctionStatements(fn)
	if err != nil {
		return err
	}

	parentLocals := c.localsMap
	parentLocalIdx := c.nextLocalIdx
	defer func() {
		c.localsMap = parentLocals
		c.nextLocalIdx = parentLocalIdx
	}()

	c.localsMap = make(map[string]ID)
	c.nextLocalIdx = 0
	block.LocalCount = 0

	return c.CompileBlock(block, stmts)
}