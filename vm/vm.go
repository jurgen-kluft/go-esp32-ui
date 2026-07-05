package vm

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	defaultDataStackCapacity = 256
	defaultLocalArenaSize    = 256
	defaultTempArenaSize     = 256
	defaultScratchSize       = 16
)

// VMBlock represents the loaded executable chunks inside the VM
type VMBlock struct {
	ID              uint32
	Scope           BlockScope
	InheritedLocals uint32
	LocalCount      uint32
	Bytes           []byte
	Consts          []Var
}

// CallFrame tracks the context of an executing block
type CallFrame struct {
	BlockID      uint32
	PC           uint32
	FramePointer uint32 // Index in the data stack where this block's locals begin
	StackBase    uint32
	LocalMark    uint32
	TempMark     uint32
}

type VM struct {
	Blocks  map[uint32]VMBlock
	Globals []Var

	// Evaluation and Storage Stack
	DataStack  []*Var
	TempArena  []Var
	LocalArena []Var
	tempTop    uint32
	localTop   uint32

	SyscallArgsScratch [defaultScratchSize]*Var
	ReturnScratch      [defaultScratchSize]*Var

	// Frame Call Stack
	CallStack    []CallFrame
	CurrentFrame CallFrame

	SysCalls VmSystemInterface
}

func NewVirtualMachine(systemCalls VmSystemInterface, globals []Var) *VM {
	return &VM{
		Blocks:     make(map[uint32]VMBlock),
		Globals:    globals,
		DataStack:  make([]*Var, 0, defaultDataStackCapacity),
		LocalArena: make([]Var, defaultLocalArenaSize),
		TempArena:  make([]Var, defaultTempArenaSize),
		CallStack:  make([]CallFrame, 0, 16),
		SysCalls:   systemCalls,
	}
}

func (vm *VM) allocTempVar(varType VarType, flags VarFlag, value any) *Var {
	if vm.tempTop >= uint32(len(vm.TempArena)) {
		panic("vm temp arena exhausted")
	}
	slot := &vm.TempArena[vm.tempTop]
	slot.Index = uint16(vm.tempTop + 1)
	slot.Type = varType
	slot.Flags = flags | VarFlagTemp
	slot.Value = value
	vm.tempTop++
	return slot
}

func (vm *VM) allocLocalVar(localIndex uint32) *Var {
	if vm.localTop >= uint32(len(vm.LocalArena)) {
		panic("vm local arena exhausted")
	}
	slot := &vm.LocalArena[vm.localTop]
	slot.Index = uint16(localIndex)
	slot.Type = VarTypeU32
	slot.Flags = VarFlagNone
	slot.Value = uint32(0)
	vm.localTop++
	return slot
}

func (vm *VM) pushVar(variable *Var) {
	vm.DataStack = append(vm.DataStack, variable)
}

func (vm *VM) popVar() *Var {
	topIdx := len(vm.DataStack) - 1
	variable := vm.DataStack[topIdx]
	vm.DataStack = vm.DataStack[:topIdx]
	return variable
}

func (vm *VM) globalVar(index uint32) (*Var, bool) {
	if index >= uint32(len(vm.Globals)) {
		return nil, false
	}
	variable := &vm.Globals[index]
	return variable, variable.IsSet()
}

func (vm *VM) constVar(block VMBlock, index uint32) (*Var, bool) {
	if index >= uint32(len(block.Consts)) {
		return nil, false
	}
	return &block.Consts[index], true
}

func (vm *VM) resolveVarRef(block VMBlock, rawID uint32) (*Var, bool) {
	ref := UnpackVarRef(rawID)
	switch ref.Storage {
	case GlobalRefType:
		return vm.globalVar(ref.Index)
	case ConstRefType:
		return vm.constVar(block, ref.Index)
	case LocalRefType:
		stackIndex := vm.CurrentFrame.FramePointer + ref.Index
		if stackIndex >= uint32(len(vm.DataStack)) {
			return nil, false
		}
		return vm.DataStack[stackIndex], true
	default:
		return nil, false
	}
}

func (vm *VM) binaryResultType(left, right *Var) VarType {
	if isFloatVarType(left.Type) || isFloatVarType(right.Type) {
		return VarTypeF32
	}
	if isSignedVarType(left.Type) || isSignedVarType(right.Type) {
		return VarTypeS32
	}
	return VarTypeU32
}

func (vm *VM) comparisonResult(ok bool) *Var {
	if ok {
		return vm.allocTempVar(VarTypeBool, VarFlagNone, true)
	}
	return vm.allocTempVar(VarTypeBool, VarFlagNone, false)
}

func (vm *VM) applyBinaryOp(opType BinaryOpType, left, right *Var) *Var {
	resultType := vm.binaryResultType(left, right)

	switch opType {
	case OpAdd:
		switch resultType {
		case VarTypeF32:
			return vm.allocTempVar(VarTypeF32, VarFlagNone, left.AsFloat32()+right.AsFloat32())
		case VarTypeS32:
			return vm.allocTempVar(VarTypeS32, VarFlagNone, left.AsInt32()+right.AsInt32())
		default:
			return vm.allocTempVar(VarTypeU32, VarFlagNone, left.AsUint32()+right.AsUint32())
		}
	case OpSub:
		switch resultType {
		case VarTypeF32:
			return vm.allocTempVar(VarTypeF32, VarFlagNone, left.AsFloat32()-right.AsFloat32())
		case VarTypeS32:
			return vm.allocTempVar(VarTypeS32, VarFlagNone, left.AsInt32()-right.AsInt32())
		default:
			return vm.allocTempVar(VarTypeU32, VarFlagNone, left.AsUint32()-right.AsUint32())
		}
	case OpMul:
		switch resultType {
		case VarTypeF32:
			return vm.allocTempVar(VarTypeF32, VarFlagNone, left.AsFloat32()*right.AsFloat32())
		case VarTypeS32:
			return vm.allocTempVar(VarTypeS32, VarFlagNone, left.AsInt32()*right.AsInt32())
		default:
			return vm.allocTempVar(VarTypeU32, VarFlagNone, left.AsUint32()*right.AsUint32())
		}
	case OpDiv:
		switch resultType {
		case VarTypeF32:
			leftValue := left.AsFloat32()
			rightValue := right.AsFloat32()
			if rightValue == 0 {
				if leftValue < 0 {
					return vm.allocTempVar(VarTypeF32, VarFlagNone, float32(math.Inf(-1)))
				}
				return vm.allocTempVar(VarTypeF32, VarFlagNone, float32(math.Inf(1)))
			}
			return vm.allocTempVar(VarTypeF32, VarFlagNone, leftValue/rightValue)
		case VarTypeS32:
			leftValue := left.AsInt32()
			rightValue := right.AsInt32()
			if rightValue == 0 {
				if leftValue < 0 {
					return vm.allocTempVar(VarTypeS32, VarFlagNone, int32(-1<<31))
				}
				return vm.allocTempVar(VarTypeS32, VarFlagNone, int32(1<<31-1))
			}
			return vm.allocTempVar(VarTypeS32, VarFlagNone, leftValue/rightValue)
		default:
			leftValue := left.AsUint32()
			rightValue := right.AsUint32()
			if rightValue == 0 {
				return vm.allocTempVar(VarTypeU32, VarFlagNone, uint32(math.MaxUint32))
			}
			return vm.allocTempVar(VarTypeU32, VarFlagNone, leftValue/rightValue)
		}
	case OpEQ:
		switch resultType {
		case VarTypeF32:
			return vm.comparisonResult(left.AsFloat32() == right.AsFloat32())
		case VarTypeS32:
			return vm.comparisonResult(left.AsInt32() == right.AsInt32())
		default:
			return vm.comparisonResult(left.AsUint32() == right.AsUint32())
		}
	case OpG:
		switch resultType {
		case VarTypeF32:
			return vm.comparisonResult(left.AsFloat32() > right.AsFloat32())
		case VarTypeS32:
			return vm.comparisonResult(left.AsInt32() > right.AsInt32())
		default:
			return vm.comparisonResult(left.AsUint32() > right.AsUint32())
		}
	case OpGE:
		switch resultType {
		case VarTypeF32:
			return vm.comparisonResult(left.AsFloat32() >= right.AsFloat32())
		case VarTypeS32:
			return vm.comparisonResult(left.AsInt32() >= right.AsInt32())
		default:
			return vm.comparisonResult(left.AsUint32() >= right.AsUint32())
		}
	case OpL:
		switch resultType {
		case VarTypeF32:
			return vm.comparisonResult(left.AsFloat32() < right.AsFloat32())
		case VarTypeS32:
			return vm.comparisonResult(left.AsInt32() < right.AsInt32())
		default:
			return vm.comparisonResult(left.AsUint32() < right.AsUint32())
		}
	case OpLE:
		switch resultType {
		case VarTypeF32:
			return vm.comparisonResult(left.AsFloat32() <= right.AsFloat32())
		case VarTypeS32:
			return vm.comparisonResult(left.AsInt32() <= right.AsInt32())
		default:
			return vm.comparisonResult(left.AsUint32() <= right.AsUint32())
		}
	default:
		return vm.allocTempVar(VarTypeU32, VarFlagNone, uint32(0))
	}
}

func (vm *VM) ExecuteBlock(blockID uint32) {
	vm.DataStack = vm.DataStack[:0]
	vm.CallStack = vm.CallStack[:0]
	vm.tempTop = 0
	vm.localTop = 0
	vm.CurrentFrame = vm.enterBlock(blockID, nil)
	vm.executeCurrentFrame()
}

func (vm *VM) enterBlock(blockID uint32, parent *CallFrame) CallFrame {
	block := vm.Blocks[blockID]
	frame := CallFrame{
		BlockID:   blockID,
		PC:        0,
		StackBase: uint32(len(vm.DataStack)),
		LocalMark: vm.localTop,
		TempMark:  vm.tempTop,
	}

	if block.Scope == BlockScopeSub && parent != nil {
		frame.FramePointer = parent.FramePointer
	} else {
		frame.FramePointer = uint32(len(vm.DataStack))
	}

	visibleLocals := block.LocalCount
	if visibleLocals < block.InheritedLocals {
		panic("vm block local metadata invalid")
	}
	newLocalCount := visibleLocals - block.InheritedLocals

	for i := uint32(0); i < newLocalCount; i++ {
		vm.pushVar(vm.allocLocalVar(block.InheritedLocals + i))
	}

	return frame
}

func (vm *VM) executeChildBlock(blockID uint32) {
	vm.CallStack = append(vm.CallStack, vm.CurrentFrame)
	parentFrame := vm.CurrentFrame
	vm.CurrentFrame = vm.enterBlock(blockID, &parentFrame)
	vm.executeSubLoop()
}

func (vm *VM) executeCurrentFrame() {
	for {
		block := vm.Blocks[vm.CurrentFrame.BlockID]
		if vm.CurrentFrame.PC >= uint32(len(block.Bytes)) {
			fmt.Println("VM Error: Program Counter ran out of bounds")
			return
		}

		op := Opcode(block.Bytes[vm.CurrentFrame.PC])
		vm.CurrentFrame.PC++

		switch op {
		case OpPushVar:
			varID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			if variable, ok := vm.resolveVarRef(block, varID); ok {
				vm.pushVar(variable)
				break
			}
			fmt.Printf("VM Error: Var reference %d not found\n", varID)
			return

		case OpPopVar:
			varID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			val := vm.popVar()
			ref := UnpackVarRef(varID)
			if ref.Storage != GlobalRefType {
				fmt.Printf("VM Error: Cannot write through non-global var reference %d\n", varID)
				return
			}
			globalVar, ok := vm.globalVar(ref.Index)
			if !ok {
				fmt.Printf("VM Error: Failed to set global variable ID %d\n", varID)
				return
			}
			globalVar.Type = val.Type
			globalVar.Flags = val.Flags
			globalVar.Value = val.Value

		case OpGetLocal:
			localRef := UnpackVarRef(binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:]))
			vm.CurrentFrame.PC += 4
			val := vm.DataStack[vm.CurrentFrame.FramePointer+localRef.Index]
			vm.pushVar(val)

		case OpSetLocal:
			localRef := UnpackVarRef(binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:]))
			vm.CurrentFrame.PC += 4
			val := vm.popVar()
			localVar := vm.DataStack[vm.CurrentFrame.FramePointer+localRef.Index]
			localVar.Type = val.Type
			localVar.Flags = val.Flags
			localVar.Value = val.Value

		case OpBinaryOp:
			opType := BinaryOpType(block.Bytes[vm.CurrentFrame.PC])
			vm.CurrentFrame.PC++

			right := vm.popVar()
			left := vm.popVar()
			vm.pushVar(vm.applyBinaryOp(opType, left, right))

		case OpIf:
			condBlockID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			trueBlockID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			falseBlockID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4

			// 1. Solve the condition in a sub-scope that reuses the current frame.
			vm.executeChildBlock(condBlockID)

			// 3. Capture the condition boolean left on the top of the stack
			condResult := vm.popVar()

			// 4. Choose which structural body execution pipeline block to execute next
			targetBlockID := falseBlockID
			if condResult.Uint32Value() != 0 {
				targetBlockID = trueBlockID
			}

			// 2. Execute the chosen branch in its own sub-scope marks.
			vm.executeChildBlock(targetBlockID)

		case OpSyscall:
			sysID := block.Bytes[vm.CurrentFrame.PC]
			vm.CurrentFrame.PC++
			argCount := block.Bytes[vm.CurrentFrame.PC]
			vm.CurrentFrame.PC++

			vm.ExecuteSyscallBlock(sysID, argCount)

		case OpReturn:
			returnCount := block.Bytes[vm.CurrentFrame.PC]
			vm.CurrentFrame.PC++
			if int(returnCount) > len(vm.ReturnScratch) {
				panic("vm return scratch exhausted")
			}

			// Extract returned values sitting on the very top of the stack frame
			retValues := vm.ReturnScratch[:returnCount]
			for i := int(returnCount) - 1; i >= 0; i-- {
				retValues[i] = vm.popVar()
			}

			// Clear out the entire local variable space allocated to this frame pointer context
			vm.DataStack = vm.DataStack[:vm.CurrentFrame.StackBase]
			vm.localTop = vm.CurrentFrame.LocalMark
			vm.tempTop = vm.CurrentFrame.TempMark

			// Push returned values back onto parent stack frame area
			for _, val := range retValues {
				vm.pushVar(val)
			}
			return
		}
	}
}

// Inline helper loop to safely step over branching frames nested in execution structures
func (vm *VM) executeSubLoop() {
	vm.executeCurrentFrame()
	if len(vm.CallStack) == 0 {
		return
	}

	vm.CurrentFrame = vm.CallStack[len(vm.CallStack)-1]
	vm.CallStack = vm.CallStack[:len(vm.CallStack)-1]
}

func (vm *VM) ExecuteSyscallBlock(sysID uint8, argCount uint8) {
	if int(argCount) > len(vm.SyscallArgsScratch) {
		panic("vm syscall scratch exhausted")
	}
	// Extract explicit argument items out of stack memory
	args := vm.SyscallArgsScratch[:argCount]
	for i := int(argCount) - 1; i >= 0; i-- {
		args[i] = vm.popVar()
	}

	switch sysID {
	case uint8(SystemCallDrawBackground):
		vm.SysCalls.DrawBackground(args[0].AsUint32())

	case uint8(SystemCallDrawSprite):
		vm.SysCalls.DrawSprite(args[0].AsUint32(), args[1].AsUint32(), args[2].AsUint32())

	case uint8(SystemCallDrawText):
		vm.SysCalls.DrawText(args[0].AsUint32(), args[1].AsString(), args[2].AsUint32(), args[3].AsUint32(), args[4].AsUint32())

	case uint8(SystemCallDrawVar):
		vm.SysCalls.DrawVar(args[0].AsUint32(), args[1].AsUint32(), args[2].AsUint32(), args[3].AsUint32(), args[4].AsUint32())

	case uint8(SystemCallStartTimer):
		timerID := args[0]
		duration := args[1]
		vm.SysCalls.StartTimer(timerID.AsUint32(), duration.AsUint32())

	case uint8(SystemCallStopTimer):
		timerID := args[0]
		vm.SysCalls.StopTimer(timerID.AsUint32())

	case uint8(SystemCallIsTimerDone):
		timerID := args[0]
		isDone := vm.SysCalls.IsTimerDone(timerID.AsUint32())
		vm.pushVar(vm.allocTempVar(VarTypeBool, VarFlagNone, isDone))

	case uint8(SystemCallSetLightOnOff):
		lightID := args[0]
		onOff := args[1]
		vm.SysCalls.SetLightOnOff(lightID.AsUint32(), onOff.AsUint32())

	case uint8(SystemCallIsLightOn):
		lightID := args[0]
		status := vm.SysCalls.IsLightOn(lightID.AsUint32())
		vm.pushVar(vm.allocTempVar(VarTypeBool, VarFlagNone, status))

	case uint8(SystemCallSetLightBrightness):
		lightID := args[0]
		brightness := args[1]
		vm.SysCalls.SetLightBrightness(lightID.AsUint32(), brightness.AsUint32())

	case uint8(SystemCallGetLightBrightness):
		lightID := args[0]
		brightness := vm.SysCalls.GetLightBrightness(lightID.AsUint32())
		vm.pushVar(vm.allocTempVar(VarTypeU32, VarFlagNone, brightness))

	case uint8(SystemCallSetLightColor):
		lightID := args[0]
		color := args[1]
		vm.SysCalls.SetLightColor(lightID.AsUint32(), color.AsUint32())

	case uint8(SystemCallGetLightColor):
		lightID := args[0]
		color := vm.SysCalls.GetLightColor(lightID.AsUint32())
		vm.pushVar(vm.allocTempVar(VarTypeU32, VarFlagNone, color))

	default:
		fmt.Printf("VM Warning: Triggered unregistered system block execution ID %d\n", sysID)
	}
}
