package vm

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
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

// ParseDeclaredGlobals collects package-level Var declarations.
func ParseDeclaredGlobals(file *ast.File) (map[string]Var, error) {
	globals := make(map[string]Var)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for idx, name := range valueSpec.Names {
				if name == nil || idx >= len(valueSpec.Values) {
					continue
				}
				variable, ok, err := parseDeclaredVar(valueSpec.Values[idx])
				if err != nil {
					return nil, err
				}
				if !ok {
					continue
				}
				globals[name.Name] = variable
			}
		}
	}

	return globals, nil
}

func parseDeclaredVar(expr ast.Expr) (Var, bool, error) {
	composite, ok := expr.(*ast.CompositeLit)
	if !ok {
		return Var{}, false, nil
	}
	if !isVarTypeExpr(composite.Type) {
		return Var{}, false, nil
	}

	variable := Var{Flags: VarFlagNone}
	for _, elt := range composite.Elts {
		field, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return Var{}, false, fmt.Errorf("declared var fields must use key:value syntax")
		}
		keyIdent, ok := field.Key.(*ast.Ident)
		if !ok {
			return Var{}, false, fmt.Errorf("declared var field key must be an identifier")
		}

		switch keyIdent.Name {
		case "Index":
			index, err := parseUint16Literal(field.Value)
			if err != nil {
				return Var{}, false, err
			}
			variable.Index = index
		case "Type":
			varType, err := parseVarTypeExpr(field.Value)
			if err != nil {
				return Var{}, false, err
			}
			variable.Type = varType
		case "Flags":
			flags, err := parseVarFlagExpr(field.Value)
			if err != nil {
				return Var{}, false, err
			}
			variable.Flags = flags
		case "Value":
			value, err := parseDeclaredVarValue(field.Value)
			if err != nil {
				return Var{}, false, err
			}
			variable.Value = value
		}
	}

	return variable, true, nil
}

func isVarTypeExpr(expr ast.Expr) bool {
	switch typedExpr := expr.(type) {
	case *ast.Ident:
		return typedExpr.Name == "Var"
	case *ast.SelectorExpr:
		return typedExpr.Sel != nil && typedExpr.Sel.Name == "Var"
	default:
		return false
	}
}

func parseUint16Literal(expr ast.Expr) (uint16, error) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, fmt.Errorf("index must be an integer literal")
	}
	value, err := strconv.ParseUint(lit.Value, 0, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid uint16 literal %q: %w", lit.Value, err)
	}
	return uint16(value), nil
}

func parseVarTypeExpr(expr ast.Expr) (VarType, error) {
	name := exprName(expr)
	switch name {
	case "VarTypeU8":
		return VarTypeU8, nil
	case "VarTypeU16":
		return VarTypeU16, nil
	case "VarTypeU32":
		return VarTypeU32, nil
	case "VarTypeS8":
		return VarTypeS8, nil
	case "VarTypeS16":
		return VarTypeS16, nil
	case "VarTypeS32":
		return VarTypeS32, nil
	case "VarTypeF32":
		return VarTypeF32, nil
	case "VarTypeStr":
		return VarTypeStr, nil
	case "VarTypeBool":
		return VarTypeBool, nil
	default:
		return 0, fmt.Errorf("unsupported VarType %q", name)
	}
}

func parseVarFlagExpr(expr ast.Expr) (VarFlag, error) {
	switch flagExpr := expr.(type) {
	case *ast.BinaryExpr:
		if flagExpr.Op != token.OR {
			return 0, fmt.Errorf("unsupported VarFlag expression")
		}
		left, err := parseVarFlagExpr(flagExpr.X)
		if err != nil {
			return 0, err
		}
		right, err := parseVarFlagExpr(flagExpr.Y)
		if err != nil {
			return 0, err
		}
		return left | right, nil
	default:
		name := exprName(expr)
		switch name {
		case "VarFlagNone", "":
			return VarFlagNone, nil
		case "VarFlagConst":
			return VarFlagConst, nil
		case "VarFlagPtr":
			return VarFlagPtr, nil
		default:
			return 0, fmt.Errorf("unsupported VarFlag %q", name)
		}
	}
}

func parseDeclaredVarValue(expr ast.Expr) (any, error) {
	switch valueExpr := expr.(type) {
	case *ast.BasicLit:
		switch valueExpr.Kind {
		case token.INT:
			value, err := strconv.ParseInt(valueExpr.Value, 0, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid integer literal %q: %w", valueExpr.Value, err)
			}
			return int32(value), nil
		case token.FLOAT:
			value, err := strconv.ParseFloat(valueExpr.Value, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid float literal %q: %w", valueExpr.Value, err)
			}
			return float32(value), nil
		case token.STRING:
			value, err := strconv.Unquote(valueExpr.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid string literal %q: %w", valueExpr.Value, err)
			}
			return value, nil
		}
	case *ast.Ident:
		switch valueExpr.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	case *ast.CallExpr:
		if len(valueExpr.Args) != 1 {
			break
		}
		return parseDeclaredVarValue(valueExpr.Args[0])
	}
	return nil, fmt.Errorf("unsupported declared var value")
}

func exprName(expr ast.Expr) string {
	switch namedExpr := expr.(type) {
	case *ast.Ident:
		return namedExpr.Name
	case *ast.SelectorExpr:
		if namedExpr.Sel == nil {
			return ""
		}
		return namedExpr.Sel.Name
	default:
		return ""
	}
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
	parentLocalVars := c.localVars
	parentLocalIdx := c.nextLocalIdx
	defer func() {
		c.localsMap = parentLocals
		c.localVars = parentLocalVars
		c.nextLocalIdx = parentLocalIdx
	}()

	c.localsMap = make(map[string]VarRef)
	c.localVars = make(map[string]Var)
	c.nextLocalIdx = 0
	block.LocalCount = 0

	return c.CompileBlock(block, stmts)
}