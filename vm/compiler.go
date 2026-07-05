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

type ID struct {
	Type uint8
	Idx  uint32
}

type Block struct {
	ID         uint32
	LocalCount uint32 // Tells the VM how many local variable slots to allocate on the stack frame
	Bytes      []byte
}

type Compiler struct {
	blocks       map[uint32]*Block
	nextBlockID  uint32
	globalVarMap map[string]ID // Predefined external variables [Type:8, Index:24]

	systemInterface CompilerSystemInterface
	// Track local variables currently in scope for the block being compiled
	localsMap    map[string]ID // [Type:8, Index:24]
	nextLocalIdx uint32
}

func NewCompiler(globals map[string]ID, systemInterface CompilerSystemInterface) *Compiler {
	c := &Compiler{
		blocks:          make(map[uint32]*Block),
		nextBlockID:     0,
		globalVarMap:    globals,
		systemInterface: systemInterface,
		localsMap:       make(map[string]ID),
		nextLocalIdx:    0,
	}
	return c
}

func (c *Compiler) AllocateBlock() *Block {
	id := c.nextBlockID
	c.nextBlockID++
	b := &Block{ID: id, LocalCount: 0, Bytes: make([]byte, 0)}
	c.blocks[id] = b
	return b
}

func writePackedID(buf *bytes.Buffer, id ID) error {
	if id.Idx > idIndexMask {
		return fmt.Errorf("id index %d exceeds 24-bit limit", id.Idx)
	}
	return binary.Write(buf, binary.LittleEndian, id.Pack())
}

func writeConstValue(buf *bytes.Buffer, constType ConstType, value any) error {
	buf.WriteByte(byte(OpPushConst))
	buf.WriteByte(byte(constType))
	return binary.Write(buf, binary.LittleEndian, value)
}

func writeUnsignedConst(buf *bytes.Buffer, value uint32) error {
	switch {
	case value <= math.MaxUint8:
		return writeConstValue(buf, ConstTypeU8, uint8(value))
	case value <= math.MaxUint16:
		return writeConstValue(buf, ConstTypeU16, uint16(value))
	default:
		return writeConstValue(buf, ConstTypeU32, value)
	}
}

func writeSignedConst(buf *bytes.Buffer, value int32) error {
	switch {
	case value >= math.MinInt8 && value <= math.MaxInt8:
		return writeConstValue(buf, ConstTypeS8, int8(value))
	case value >= math.MinInt16 && value <= math.MaxInt16:
		return writeConstValue(buf, ConstTypeS16, int16(value))
	default:
		return writeConstValue(buf, ConstTypeS32, value)
	}
}

func writeFloatConst(buf *bytes.Buffer, value float64) error {
	return writeConstValue(buf, ConstTypeF32, math.Float32bits(float32(value)))
}

func (c *Compiler) CompileBlock(block *Block, stmts []ast.Stmt) error {
	var buf bytes.Buffer
	hasExplicitReturn := false

	// Backup parent scope context if entering a sub-block/branch
	parentLocals := make(map[string]ID)
	for k, v := range c.localsMap {
		parentLocals[k] = v
	}
	parentLocalIdx := c.nextLocalIdx

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
				if globalID, isGlobal := c.globalVarMap[name]; isGlobal {
					buf.WriteByte(byte(OpPopVar))
					if err := writePackedID(&buf, globalID); err != nil {
						return err
					}
					continue
				}

				// Branch B: Target is a local stack variable
				localIdx, isRegistered := c.localsMap[name]
				if !isRegistered {
					// Register a new local variable slot
					localIdx = ID{Type: LocalIDType, Idx: c.nextLocalIdx}
					c.localsMap[name] = localIdx
					c.nextLocalIdx++

					// Keep track of the highest local slot count reached in this block context
					if c.nextLocalIdx > block.LocalCount {
						block.LocalCount = c.nextLocalIdx
					}
				}

				buf.WriteByte(byte(OpSetLocal))
				if err := writePackedID(&buf, localIdx); err != nil {
					return err
				}
			}

		case *ast.IfStmt:
			condBlock := c.AllocateBlock()
			trueBlock := c.AllocateBlock()
			falseBlock := c.AllocateBlock()

			// Sub-blocks automatically inherit access to parent local registries
			condBytes, err := c.compileExpression(s.Cond)
			if err != nil {
				return err
			}
			condBlock.Bytes = append(condBytes, byte(OpReturn), 1)

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

	// Restore parent local scope tracking boundaries upon exit
	c.localsMap = parentLocals
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
			if err := writeUnsignedConst(&buf, uint32(val)); err != nil {
				return nil, err
			}
		}
		if e.Kind == token.FLOAT {
			val, err := strconv.ParseFloat(e.Value, 32)
			if err != nil {
				return nil, fmt.Errorf("unsupported float literal %q: %w", e.Value, err)
			}
			if err := writeFloatConst(&buf, val); err != nil {
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
					if err := writeSignedConst(&buf, int32(val)); err != nil {
						return nil, err
					}
				case token.FLOAT:
					val, err := strconv.ParseFloat("-"+lit.Value, 32)
					if err != nil {
						return nil, fmt.Errorf("unsupported float literal -%s: %w", lit.Value, err)
					}
					if err := writeFloatConst(&buf, val); err != nil {
						return nil, err
					}
				default:
					return nil, fmt.Errorf("unsupported unary literal: %s", lit.Kind)
				}
				return buf.Bytes(), nil
			}

			if err := writeUnsignedConst(&buf, 0); err != nil {
				return nil, err
			}
			operandBytes, err := c.compileExpression(e.X)
			if err != nil {
				return nil, err
			}
			buf.Write(operandBytes)
			buf.WriteByte(byte(OpBinaryOp))
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
			if err := writePackedID(&buf, localIdx); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}

		// 2. Check if it maps to a global predefined variable ID
		if globalID, isGlobal := c.globalVarMap[name]; isGlobal {
			buf.WriteByte(byte(OpPushVar))
			if err := writePackedID(&buf, globalID); err != nil {
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
		buf.WriteByte(byte(OpBinaryOp))
		switch e.Op {
		case token.ADD:
			buf.WriteByte(byte(OpAdd))
		case token.SUB:
			buf.WriteByte(byte(OpSub))
		case token.MUL:
			buf.WriteByte(byte(OpMul))
		case token.QUO:
			buf.WriteByte(byte(OpDiv))
		case token.EQL:
			buf.WriteByte(byte(OpEQ))
		case token.GTR:
			buf.WriteByte(byte(OpG))
		case token.GEQ:
			buf.WriteByte(byte(OpGE))
		case token.LSS:
			buf.WriteByte(byte(OpL))
		case token.LEQ:
			buf.WriteByte(byte(OpLE))
		default:
			return nil, fmt.Errorf("unsupported binary operator: %s", e.Op)
		}

	case *ast.CallExpr:
		ident, ok := e.Fun.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("dynamic pointer targets forbidden")
		}
		sysID, ok := c.systemInterface.RegisterSystemCall(ident.Name)
		if !ok {
			return nil, fmt.Errorf("unknown syscall: %s", ident.Name)
		}
		for _, arg := range e.Args {
			argBytes, err := c.compileExpression(arg)
			if err != nil {
				return nil, err
			}
			buf.Write(argBytes)
		}
		buf.WriteByte(byte(OpSyscall))
		buf.WriteByte(sysID)
		buf.WriteByte(uint8(len(e.Args)))
	}

	return buf.Bytes(), nil
}
