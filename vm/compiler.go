package vm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"go/ast"
	"go/token"
)

type Block struct {
	ID         uint32
	LocalCount uint32 // Tells the VM how many local variable slots to allocate on the stack frame
	Bytes      []byte
}

type Compiler struct {
	blocks       map[uint32]*Block
	nextBlockID  uint32
	syscallMap   map[string]uint8
	globalVarMap map[string]uint32 // Predefined external variables [Type:8, Index:24]

	// Track local variables currently in scope for the block being compiled
	localsMap    map[string]uint32 // [Type:8, Index:24]
	nextLocalIdx uint32
}

func NewCompiler(globals map[string]uint32) *Compiler {
	c := &Compiler{
		blocks:       make(map[uint32]*Block),
		nextBlockID:  0,
		syscallMap:   make(map[string]uint8),
		globalVarMap: globals,
		localsMap:    make(map[string]uint32),
		nextLocalIdx: 0,
	}
	c.initBuiltins()
	return c
}

func (c *Compiler) initBuiltins() {
	c.syscallMap["IsLightOn"] = 1
	c.syscallMap["SetLightOnOff"] = 2
	c.syscallMap["DrawSprite"] = 3
	c.syscallMap["DrawText"] = 4
}

func (c *Compiler) AllocateBlock() *Block {
	id := c.nextBlockID
	c.nextBlockID++
	b := &Block{ID: id, LocalCount: 0, Bytes: make([]byte, 0)}
	c.blocks[id] = b
	return b
}

func (c *Compiler) CompileBlock(block *Block, stmts []ast.Stmt) error {
	var buf bytes.Buffer
	hasExplicitReturn := false

	// Backup parent scope context if entering a sub-block/branch
	parentLocals := make(map[string]uint32)
	for k, v := range c.localsMap {
		parentLocals[k] = v
	}
	parentLocalIdx := c.nextLocalIdx

	for _, stmt := range stmts {
		switch s := stmt.(type) {

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
					binary.Write(&buf, binary.LittleEndian, globalID)
					continue
				}

				// Branch B: Target is a local stack variable
				localIdx, isRegistered := c.localsMap[name]
				if !isRegistered {
					// Register a new local variable slot
					localIdx = c.nextLocalIdx
					c.localsMap[name] = localIdx
					c.nextLocalIdx++

					// Keep track of the highest local slot count reached in this block context
					if c.nextLocalIdx > block.LocalCount {
						block.LocalCount = c.nextLocalIdx
					}
				}

				buf.WriteByte(byte(OpSetLocal))
				binary.Write(&buf, binary.LittleEndian, localIdx)
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
	case *ast.BasicLit:
		if e.Kind == token.INT {
			var val uint32
			fmt.Sscanf(e.Value, "%d", &val)
			if val <= 255 {
				buf.WriteByte(byte(OpPushConstU8))
				buf.WriteByte(byte(val))
			} else {
				buf.WriteByte(byte(OpPushConstU32))
				binary.Write(&buf, binary.LittleEndian, val)
			}
		}

	case *ast.Ident:
		name := e.Name

		// 1. Check if the identifier maps to an active local stack variable slot
		if localIdx, isLocal := c.localsMap[name]; isLocal {
			buf.WriteByte(byte(OpGetLocal))
			binary.Write(&buf, binary.LittleEndian, localIdx)
			return buf.Bytes(), nil
		}

		// 2. Check if it maps to a global predefined variable ID
		if globalID, isGlobal := c.globalVarMap[name]; isGlobal {
			buf.WriteByte(byte(OpPushVar))
			binary.Write(&buf, binary.LittleEndian, globalID)
			return buf.Bytes(), nil
		}

		return nil, fmt.Errorf("compile error: variable '%s' undefined", name)

	case *ast.BinaryExpr:
		left, _ := c.compileExpression(e.X)
		right, _ := c.compileExpression(e.Y)
		buf.Write(left)
		buf.Write(right)
		buf.WriteByte(byte(OpBinaryOp))
		switch e.Op {
		case token.ADD:
			buf.WriteByte(byte(OpAdd))
		case token.EQL:
			buf.WriteByte(byte(OpEqual))
		}

	case *ast.CallExpr:
		ident, ok := e.Fun.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("dynamic pointer targets forbidden")
		}
		sysID, ok := c.syscallMap[ident.Name]
		if !ok {
			return nil, fmt.Errorf("unknown syscall: %s", ident.Name)
		}
		for _, arg := range e.Args {
			argBytes, _ := c.compileExpression(arg)
			buf.Write(argBytes)
		}
		buf.WriteByte(byte(OpSyscall))
		buf.WriteByte(sysID)
		buf.WriteByte(uint8(len(e.Args)))
	}

	return buf.Bytes(), nil
}
