package vm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/token"
	"math"
	"strconv"
)

type BlockScope uint8

const (
	BlockScopeFrame BlockScope = iota
	BlockScopeSub
)

type Block struct {
	ID              uint32
	Scope           BlockScope
	InheritedLocals uint32
	LocalCount      uint32 // Tells the VM how many local variable slots are visible in this block
	Bytes           []byte
	Consts          []Var
}

type Compiler struct {
	blocks          map[uint32]*Block
	nextBlockID     uint32
	explicitGlobals map[string]VarRef

	systemCalls      map[string]uint8
	declaredGlobals  map[string]Var
	numericConstants map[string]Var
	stringConstants  map[string]Var
	nextConstIndex   uint16
	// Track local variables currently in scope for the block being compiled
	localsMap    map[string]VarRef
	localVars    map[string]Var
	nextLocalIdx uint32
}

func NewCompiler(globals map[string]VarRef, systemInterface map[string]uint8) *Compiler {
	c := &Compiler{
		blocks:           make(map[uint32]*Block),
		nextBlockID:      0,
		explicitGlobals:  globals,
		systemCalls:      systemInterface,
		declaredGlobals:  make(map[string]Var),
		numericConstants: make(map[string]Var),
		stringConstants:  make(map[string]Var),
		nextConstIndex:   0,
		localsMap:        make(map[string]VarRef),
		localVars:        make(map[string]Var),
		nextLocalIdx:     0,
	}
	return c
}

func (c *Compiler) LoadDeclaredGlobalsFromFile(path string) error {
	_, file, err := ParseGoFile(path)
	if err != nil {
		return err
	}

	globals, err := ParseDeclaredGlobals(file)
	if err != nil {
		return err
	}
	for name, variable := range globals {
		c.declaredGlobals[name] = variable
	}
	return nil
}

func (c *Compiler) lookupGlobalRef(name string) (VarRef, bool) {
	if variable, ok := c.declaredGlobals[name]; ok {
		return GlobalRef(uint32(variable.Index)), true
	}
	if c.explicitGlobals == nil {
		return VarRef{}, false
	}
	ref, ok := c.explicitGlobals[name]
	return ref, ok
}

func (c *Compiler) internStringConstant(literal string) VarRef {
	if variable, ok := c.stringConstants[literal]; ok {
		return ConstRef(uint32(variable.Index))
	}

	variable := Var{
		Index: c.nextConstIndex,
		Type:  VarTypeStr,
		Flags: VarFlagConst | VarFlagPtr,
		Value: literal,
	}
	c.stringConstants[literal] = variable
	c.nextConstIndex++
	return ConstRef(uint32(variable.Index))
}

func compilerConstKey(varType VarType, bits uint32) string {
	return fmt.Sprintf("%d:%d", varType, bits)
}

func (c *Compiler) internNumericConstant(varType VarType, bits uint32) VarRef {
	key := compilerConstKey(varType, bits)
	if variable, ok := c.numericConstants[key]; ok {
		return ConstRef(uint32(variable.Index))
	}

	variable := Var{
		Index: c.nextConstIndex,
		Type:  varType,
		Flags: VarFlagConst,
	}
	variable.SetUint32Value(bits)
	c.numericConstants[key] = variable
	c.nextConstIndex++
	return ConstRef(uint32(variable.Index))
}

func unwrapCallName(expr ast.Expr) (string, bool) {
	switch call := expr.(type) {
	case *ast.Ident:
		return call.Name, true
	case *ast.IndexExpr:
		return unwrapCallName(call.X)
	case *ast.IndexListExpr:
		return unwrapCallName(call.X)
	default:
		return "", false
	}
}

func (c *Compiler) compileBinaryIntrinsic(args []ast.Expr, op Opcode) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("intrinsic expects 2 arguments")
	}

	left, err := c.compileExpression(args[0])
	if err != nil {
		return nil, err
	}
	right, err := c.compileExpression(args[1])
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(left)
	buf.Write(right)
	buf.WriteByte(byte(op))
	return buf.Bytes(), nil
}

func (c *Compiler) compileNumericCast(targetType VarType, args []ast.Expr) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("numeric cast expects 1 argument")
	}

	inner, err := c.compileExpression(args[0])
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(inner)
	if err := writePushVarRef(&buf, c.internNumericConstant(targetType, 0)); err != nil {
		return nil, err
	}
	buf.WriteByte(byte(OpAdd))
	return buf.Bytes(), nil
}

func (c *Compiler) resolveAssignIntrinsicTarget(expr ast.Expr) (Opcode, VarRef, error) {
	switch target := expr.(type) {
	case *ast.UnaryExpr:
		if target.Op != token.AND {
			return 0, VarRef{}, fmt.Errorf("VarAssign target must use &identifier")
		}
		return c.resolveAssignIntrinsicTarget(target.X)
	case *ast.Ident:
		if globalRef, isGlobal := c.lookupGlobalRef(target.Name); isGlobal {
			return OpPopVar, globalRef, nil
		}
		if localRef, isLocal := c.localsMap[target.Name]; isLocal {
			return OpSetLocal, localRef, nil
		}
		return 0, VarRef{}, fmt.Errorf("VarAssign target '%s' undefined", target.Name)
	default:
		return 0, VarRef{}, fmt.Errorf("VarAssign target must be an identifier")
	}
}

func (c *Compiler) compileAssignIntrinsic(args []ast.Expr) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("VarAssign expects 2 arguments")
	}

	opcode, ref, err := c.resolveAssignIntrinsicTarget(args[0])
	if err != nil {
		return nil, err
	}
	rhs, err := c.compileExpression(args[1])
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(rhs)
	buf.WriteByte(byte(opcode))
	if err := writePackedVarRef(&buf, ref); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *Compiler) compileIntrinsicCall(name string, args []ast.Expr) (bytes []byte, ok bool, err error) {
	ok = true
	switch name {
	case "VarToUint32", "uint32":
		bytes, err = c.compileNumericCast(VarTypeU32, args)
	case "VarToInt32", "int32":
		bytes, err = c.compileNumericCast(VarTypeS32, args)
	case "VarToFloat32", "float32":
		bytes, err = c.compileNumericCast(VarTypeF32, args)
	case "VarAssign":
		bytes, err = c.compileAssignIntrinsic(args)
	case "VarEq":
		bytes, err = c.compileBinaryIntrinsic(args, OpEQ)
	case "VarLt":
		bytes, err = c.compileBinaryIntrinsic(args, OpL)
	case "VarLe":
		bytes, err = c.compileBinaryIntrinsic(args, OpLE)
	case "VarGt":
		bytes, err = c.compileBinaryIntrinsic(args, OpG)
	case "VarGe":
		bytes, err = c.compileBinaryIntrinsic(args, OpGE)
	default:
		ok = false
	}
	return bytes, ok, err
}

func writePushVarRef(buf *bytes.Buffer, ref VarRef) error {
	buf.WriteByte(byte(OpPushVar))
	return writePackedVarRef(buf, ref)
}

func (c *Compiler) CompiledConsts() []Var {
	consts := make([]Var, c.nextConstIndex)
	for _, variable := range c.numericConstants {
		consts[variable.Index] = variable
	}
	for _, variable := range c.stringConstants {
		consts[variable.Index] = variable
	}
	return consts
}

func (c *Compiler) allocateBlock(scope BlockScope) *Block {
	id := c.nextBlockID
	c.nextBlockID++
	b := &Block{ID: id, Scope: scope, LocalCount: 0, Bytes: make([]byte, 0)}
	c.blocks[id] = b
	return b
}

func (c *Compiler) AllocateBlock() *Block {
	return c.allocateBlock(BlockScopeFrame)
}

func (c *Compiler) AllocateSubBlock() *Block {
	return c.allocateBlock(BlockScopeSub)
}

func writePackedVarRef(buf *bytes.Buffer, ref VarRef) error {
	if ref.Index > varRefIndexMask {
		return fmt.Errorf("var ref index %d exceeds 24-bit limit", ref.Index)
	}
	return binary.Write(buf, binary.LittleEndian, ref.Pack())
}

func varTypeForUnsignedLiteral(value uint32) VarType {
	switch {
	case value <= math.MaxUint8:
		return VarTypeU8
	case value <= math.MaxUint16:
		return VarTypeU16
	default:
		return VarTypeU32
	}
}

func varTypeForSignedLiteral(value int32) VarType {
	switch {
	case value >= math.MinInt8 && value <= math.MaxInt8:
		return VarTypeS8
	case value >= math.MinInt16 && value <= math.MaxInt16:
		return VarTypeS16
	default:
		return VarTypeS32
	}
}

func (c *Compiler) CompileBlock(block *Block, stmts []ast.Stmt) error {
	var buf bytes.Buffer
	hasExplicitReturn := false

	// Backup parent scope context if entering a sub-block/branch
	parentLocals := make(map[string]VarRef)
	for k, v := range c.localsMap {
		parentLocals[k] = v
	}
	parentLocalVars := make(map[string]Var)
	for k, v := range c.localVars {
		parentLocalVars[k] = v
	}
	parentLocalIdx := c.nextLocalIdx
	block.InheritedLocals = parentLocalIdx

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.ExprStmt:
			exprBytes, err := c.compileExpression(s.X)
			if err != nil {
				return err
			}
			buf.Write(exprBytes)

		case *ast.AssignStmt:
			// 1. Evaluate Right Hand Side expressions first
			for _, rhsExpr := range s.Rhs {
				rhsBytes, err := c.compileExpression(rhsExpr)
				if err != nil {
					return err
				}
				buf.Write(rhsBytes)
			}

			// 2. Map assignment targets to storage cells from RIGHT to LEFT
			for i := len(s.Lhs) - 1; i >= 0; i-- {
				lhsIdent, ok := s.Lhs[i].(*ast.Ident)
				if !ok {
					return fmt.Errorf("assignments must target identifiers")
				}

				name := lhsIdent.Name

				// Branch A: Target is a predefined global variable ID
				if globalRef, isGlobal := c.lookupGlobalRef(name); isGlobal {
					buf.WriteByte(byte(OpPopVar))
					if err := writePackedVarRef(&buf, globalRef); err != nil {
						return err
					}
					continue
				}

				// Branch B: Target is a local stack variable
				localIdx, isRegistered := c.localsMap[name]
				if !isRegistered {
					// Register a new local variable slot
					localIdx = LocalRef(c.nextLocalIdx)
					c.localsMap[name] = localIdx
					c.localVars[name] = Var{Index: uint16(localIdx.Index), Type: VarTypeU32}
					c.nextLocalIdx++

					// Keep track of the highest local slot count reached in this block context
					if c.nextLocalIdx > block.LocalCount {
						block.LocalCount = c.nextLocalIdx
					}
				}

				buf.WriteByte(byte(OpSetLocal))
				if err := writePackedVarRef(&buf, localIdx); err != nil {
					return err
				}
			}

		case *ast.IfStmt:
			condBlock := c.AllocateSubBlock()
			trueBlock := c.AllocateSubBlock()
			falseBlock := c.AllocateSubBlock()
			condBlock.InheritedLocals = c.nextLocalIdx
			condBlock.LocalCount = c.nextLocalIdx
			falseBlock.InheritedLocals = c.nextLocalIdx
			falseBlock.LocalCount = c.nextLocalIdx

			// Sub-blocks automatically inherit access to parent local registries
			condBytes, err := c.compileExpression(s.Cond)
			if err != nil {
				return err
			}
			condBlock.Bytes = append(condBytes, byte(OpReturn), 1)
			condBlock.Consts = c.CompiledConsts()

			if err := c.CompileBlock(trueBlock, s.Body.List); err != nil {
				return err
			}

			if s.Else != nil {
				if elseBlock, ok := s.Else.(*ast.BlockStmt); ok {
					if err := c.CompileBlock(falseBlock, elseBlock.List); err != nil {
						return err
					}
				}
			} else {
				falseBlock.Bytes = []byte{byte(OpReturn), 0}
			}

			buf.WriteByte(byte(OpIf))
			binary.Write(&buf, binary.LittleEndian, condBlock.ID)
			binary.Write(&buf, binary.LittleEndian, trueBlock.ID)
			binary.Write(&buf, binary.LittleEndian, falseBlock.ID)

		case *ast.ReturnStmt:
			if len(s.Results) > defaultReturnScratchSize {
				return fmt.Errorf("compile error: return supports at most %d values", defaultReturnScratchSize)
			}
			for _, rExpr := range s.Results {
				rBytes, err := c.compileExpression(rExpr)
				if err != nil {
					return err
				}
				buf.Write(rBytes)
			}
			buf.WriteByte(byte(OpReturn))
			buf.WriteByte(uint8(len(s.Results)))
			hasExplicitReturn = true
		}
	}

	if !hasExplicitReturn {
		buf.WriteByte(byte(OpReturn))
		buf.WriteByte(0)
	}

	block.Bytes = buf.Bytes()
	block.Consts = c.CompiledConsts()

	// Restore parent local scope tracking boundaries upon exit
	c.localsMap = parentLocals
	c.localVars = parentLocalVars
	c.nextLocalIdx = parentLocalIdx
	return nil
}

func (c *Compiler) compileExpression(expr ast.Expr) ([]byte, error) {
	var buf bytes.Buffer

	switch e := expr.(type) {
	case *ast.ParenExpr:
		return c.compileExpression(e.X)

	case *ast.BasicLit:
		if e.Kind == token.INT {
			val, err := strconv.ParseUint(e.Value, 0, 32)
			if err != nil {
				return nil, fmt.Errorf("unsupported int literal %q: %w", e.Value, err)
			}
			id := c.internNumericConstant(varTypeForUnsignedLiteral(uint32(val)), uint32(val))
			if err := writePushVarRef(&buf, id); err != nil {
				return nil, err
			}
		}
		if e.Kind == token.FLOAT {
			val, err := strconv.ParseFloat(e.Value, 32)
			if err != nil {
				return nil, fmt.Errorf("unsupported float literal %q: %w", e.Value, err)
			}
			id := c.internNumericConstant(VarTypeF32, math.Float32bits(float32(val)))
			if err := writePushVarRef(&buf, id); err != nil {
				return nil, err
			}
		}
		if e.Kind == token.STRING {
			literal, err := strconv.Unquote(e.Value)
			if err != nil {
				return nil, fmt.Errorf("unsupported string literal %q: %w", e.Value, err)
			}
			id := c.internStringConstant(literal)
			if err := writePushVarRef(&buf, id); err != nil {
				return nil, err
			}
		}

	case *ast.UnaryExpr:
		switch e.Op {
		case token.ADD:
			return c.compileExpression(e.X)
		case token.SUB:
			if lit, ok := e.X.(*ast.BasicLit); ok {
				switch lit.Kind {
				case token.INT:
					val, err := strconv.ParseInt("-"+lit.Value, 0, 32)
					if err != nil {
						return nil, fmt.Errorf("unsupported int literal -%s: %w", lit.Value, err)
					}
					id := c.internNumericConstant(varTypeForSignedLiteral(int32(val)), uint32(int32(val)))
					if err := writePushVarRef(&buf, id); err != nil {
						return nil, err
					}
				case token.FLOAT:
					val, err := strconv.ParseFloat("-"+lit.Value, 32)
					if err != nil {
						return nil, fmt.Errorf("unsupported float literal -%s: %w", lit.Value, err)
					}
					id := c.internNumericConstant(VarTypeF32, math.Float32bits(float32(val)))
					if err := writePushVarRef(&buf, id); err != nil {
						return nil, err
					}
				default:
					return nil, fmt.Errorf("unsupported unary literal: %s", lit.Kind)
				}
				return buf.Bytes(), nil
			}

			if err := writePushVarRef(&buf, c.internNumericConstant(VarTypeU8, 0)); err != nil {
				return nil, err
			}
			operandBytes, err := c.compileExpression(e.X)
			if err != nil {
				return nil, err
			}
			buf.Write(operandBytes)
			buf.WriteByte(byte(OpSub))
			return buf.Bytes(), nil
		default:
			return nil, fmt.Errorf("unsupported unary operator: %s", e.Op)
		}

	case *ast.Ident:
		name := e.Name

		// 1. Check if the identifier maps to an active local stack variable slot
		if localIdx, isLocal := c.localsMap[name]; isLocal {
			buf.WriteByte(byte(OpGetLocal))
			if err := writePackedVarRef(&buf, localIdx); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}

		// 2. Check if it maps to a global predefined variable ID
		if globalRef, isGlobal := c.lookupGlobalRef(name); isGlobal {
			if err := writePushVarRef(&buf, globalRef); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}

		return nil, fmt.Errorf("compile error: variable '%s' undefined", name)

	case *ast.BinaryExpr:
		left, err := c.compileExpression(e.X)
		if err != nil {
			return nil, err
		}
		right, err := c.compileExpression(e.Y)
		if err != nil {
			return nil, err
		}
		buf.Write(left)
		buf.Write(right)

		opcode, ok := tokenToOpcode(e.Op)
		if !ok {
			return nil, fmt.Errorf("unsupported binary operator: %s", e.Op)
		}
		buf.WriteByte(byte(opcode))

	case *ast.CallExpr:
		name, ok := unwrapCallName(e.Fun)
		if !ok {
			return nil, fmt.Errorf("dynamic pointer targets forbidden")
		}

		if intrinsicBytes, handled, err := c.compileIntrinsicCall(name, e.Args); handled {
			if err != nil {
				return nil, err
			}
			return intrinsicBytes, nil
		}

		sysID, ok := c.systemCalls[name]
		if !ok {
			return nil, fmt.Errorf("unknown syscall: %s", name)
		}
		for argIndex, arg := range e.Args {
			argBytes, err := c.compileExpression(arg)
			if err != nil {
				return nil, err
			}
			_ = argIndex
			buf.Write(argBytes)
		}
		buf.WriteByte(byte(OpSyscall))
		buf.WriteByte(sysID)
		buf.WriteByte(uint8(len(e.Args)))
	}

	return buf.Bytes(), nil
}
