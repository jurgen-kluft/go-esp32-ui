package vm

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const (
	defaultRefStackCapacity  = 256
	defaultLocalArenaSize    = 256
	defaultTempArenaSize     = 256
	defaultReturnScratchSize = 4
	maxSyscallArgs           = 5
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
	BlockID   uint32
	PC        uint32
	LocalBase uint32 // Index in the ref stack where this block's visible locals begin
	StackBase uint32
	LocalMark uint32
	TempMark  uint32
}

type VM struct {
	Blocks  map[uint32]VMBlock
	Globals []Var

	// Evaluation and Storage Stack
	RefStack    []StackRef
	TempArena   []Var
	LocalArena  []Var
	tempTop     uint32
	localTop    uint32
	nextScopeID uint32

	// Frame Call Stack
	CallStack    []CallFrame
	CurrentFrame CallFrame

	SysCalls VmSystemInterface
}

func NewVirtualMachine(systemCalls VmSystemInterface, globals []Var) *VM {
	return &VM{
		Blocks:      make(map[uint32]VMBlock),
		Globals:     globals,
		RefStack:    make([]StackRef, 0, defaultRefStackCapacity),
		LocalArena:  make([]Var, defaultLocalArenaSize),
		TempArena:   make([]Var, defaultTempArenaSize),
		CallStack:   make([]CallFrame, 0, 16),
		SysCalls:    systemCalls,
		nextScopeID: 1,
	}
}

func (vm *VM) allocScopeID() uint32 {
	id := vm.nextScopeID
	vm.nextScopeID++
	if id == 0 {
		panic("vm scope id exhausted")
	}
	return id
}

func (vm *VM) allocTempVar(varType VarType, flags VarFlag, value any) *Var {
	if vm.tempTop >= uint32(len(vm.TempArena)) {
		panic("vm temp arena exhausted")
	}
	tempIndex := vm.tempTop
	slot := &vm.TempArena[tempIndex]
	slot.Index = uint16(tempIndex + 1)
	slot.Type = varType
	slot.Flags = flags | VarFlagTemp
	slot.Generation = vm.allocScopeID()
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
	slot.Generation = 0
	slot.Value = uint32(0)
	vm.localTop++
	return slot
}

func (vm *VM) pushVarWithRef(_ *Var, ref StackRef) {
	vm.RefStack = append(vm.RefStack, ref)
}

func (vm *VM) StackLen() uint32 {
	return uint32(len(vm.RefStack))
}

func (vm *VM) StackValueAt(index uint32) (*Var, bool) {
	if index >= uint32(len(vm.RefStack)) {
		return nil, false
	}
	block, ok := vm.Blocks[vm.CurrentFrame.BlockID]
	if !ok {
		block = VMBlock{}
	}
	return vm.resolveStackRef(block, vm.RefStack[index])
}

func (vm *VM) pushTempVar(variable *Var) {
	tempIndex := uint32(variable.Index - 1)
	if variable.Generation == 0 {
		panic("vm temp generation missing")
	}
	vm.pushVarWithRef(variable, StackRefFromVarRef(TempRef(tempIndex), variable.Generation))
}

func (vm *VM) pushTempValue(varType VarType, flags VarFlag, value any) {
	vm.pushTempVar(vm.allocTempVar(varType, flags, value))
}

func (vm *VM) runtimeErrorf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("VM Error: %s | Stack: %s\n", message, vm.DumpRefStack())
}

func (vm *VM) truncateStacks(size uint32) {
	vm.RefStack = vm.RefStack[:size]
}

func (vm *VM) popStackRef() StackRef {
	topIdx := len(vm.RefStack) - 1
	ref := vm.RefStack[topIdx]
	vm.truncateStacks(uint32(topIdx))
	return ref
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

func (vm *VM) liveLocalRef(index uint32) (StackRef, bool) {
	stackIndex := vm.CurrentFrame.LocalBase + index
	if stackIndex >= uint32(len(vm.RefStack)) {
		return StackRef{}, false
	}
	ref := vm.RefStack[stackIndex]
	if ref.Storage != LocalRefType || ref.Index != index {
		return StackRef{}, false
	}
	return ref, true
}

func (vm *VM) localSlotVar(index uint32) (*Var, bool) {
	if index >= vm.localTop {
		return nil, false
	}
	variable := &vm.LocalArena[index]
	if uint32(variable.Index) != index {
		return nil, false
	}
	return variable, true
}

func (vm *VM) resolveLocalVar(index uint32) (*Var, bool) {
	ref, ok := vm.liveLocalRef(index)
	if !ok {
		return nil, false
	}
	return vm.localSlotVar(ref.Index)
}

func (vm *VM) tempSlotVar(index uint32) (*Var, bool) {
	if index >= vm.tempTop {
		return nil, false
	}
	return &vm.TempArena[index], true
}

func (vm *VM) liveTempGeneration(index uint32) (uint32, bool) {
	variable, ok := vm.tempSlotVar(index)
	if !ok {
		return 0, false
	}
	generation := variable.Generation
	if generation == 0 {
		return 0, false
	}
	return generation, true
}

func (vm *VM) resolveTempVar(index uint32) (*Var, bool) {
	if _, ok := vm.liveTempGeneration(index); !ok {
		return nil, false
	}
	return vm.tempSlotVar(index)
}

func (vm *VM) resolveRef(block VMBlock, ref VarRef) (*Var, bool) {
	switch ref.Storage {
	case GlobalRefType:
		return vm.globalVar(ref.Index)
	case ConstRefType:
		return vm.constVar(block, ref.Index)
	case LocalRefType:
		return vm.resolveLocalVar(ref.Index)
	case TempRefType:
		return vm.resolveTempVar(ref.Index)
	default:
		return nil, false
	}
}

func (vm *VM) resolveStackRef(block VMBlock, ref StackRef) (*Var, bool) {
	switch ref.Storage {
	case LocalRefType:
		liveRef, ok := vm.liveLocalRef(ref.Index)
		if !ok || liveRef.ScopeID != ref.ScopeID {
			return nil, false
		}
		return vm.localSlotVar(ref.Index)
	case TempRefType:
		generation, ok := vm.liveTempGeneration(ref.Index)
		if !ok || generation != ref.ScopeID {
			return nil, false
		}
		return vm.tempSlotVar(ref.Index)
	default:
		return vm.resolveRef(block, VarRef{Storage: ref.Storage, Index: ref.Index})
	}
}

func (vm *VM) resolveVarRef(block VMBlock, rawID uint32) (*Var, bool) {
	return vm.resolveRef(block, UnpackVarRef(rawID))
}

func formatStackRef(ref StackRef) string {
	switch ref.Storage {
	case GlobalRefType:
		return fmt.Sprintf("Global(%d)", ref.Index)
	case ConstRefType:
		return fmt.Sprintf("Const(%d)", ref.Index)
	case LocalRefType:
		return fmt.Sprintf("Local(%d)@%d", ref.Index, ref.ScopeID)
	case TempRefType:
		return fmt.Sprintf("Temp(%d)@%d", ref.Index, ref.ScopeID)
	default:
		return fmt.Sprintf("Unknown(%d:%d)", ref.Storage, ref.Index)
	}
}

func (vm *VM) DumpRefStack() string {
	if len(vm.RefStack) == 0 {
		return "[]"
	}
	parts := make([]string, len(vm.RefStack))
	for i, ref := range vm.RefStack {
		parts[i] = formatStackRef(ref)
	}
	return "[" + strings.Join(parts, ", ") + "]"
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

func (vm *VM) applyBinaryOp(op Opcode, left, right *Var) *Var {
	resultType := vm.binaryResultType(left, right)

	switch op {
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
	vm.truncateStacks(0)
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
		StackBase: uint32(len(vm.RefStack)),
		LocalMark: vm.localTop,
		TempMark:  vm.tempTop,
	}

	if block.Scope == BlockScopeSub && parent != nil {
		frame.LocalBase = parent.LocalBase
	} else {
		frame.LocalBase = uint32(len(vm.RefStack))
	}

	visibleLocals := block.LocalCount
	if visibleLocals < block.InheritedLocals {
		panic("vm block local metadata invalid")
	}
	newLocalCount := visibleLocals - block.InheritedLocals

	for i := uint32(0); i < newLocalCount; i++ {
		localIndex := block.InheritedLocals + i
		vm.pushVarWithRef(vm.allocLocalVar(localIndex), StackRefFromVarRef(LocalRef(localIndex), vm.allocScopeID()))
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
			vm.runtimeErrorf("program counter ran out of bounds")
			return
		}

		op := Opcode(block.Bytes[vm.CurrentFrame.PC])
		vm.CurrentFrame.PC++

		switch op {
		case OpPushVar:
			varID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			if variable, ok := vm.resolveVarRef(block, varID); ok {
				vm.pushVarWithRef(variable, StackRefFromVarRef(UnpackVarRef(varID), vm.CurrentFrame.BlockID))
				break
			}
			vm.runtimeErrorf("var reference %d not found", varID)
			return

		case OpPopVar:
			varID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			valueRef := vm.popStackRef()
			val, ok := vm.resolveStackRef(block, valueRef)
			if !ok {
				vm.runtimeErrorf("failed to resolve stack ref %+v", valueRef)
				return
			}
			ref := UnpackVarRef(varID)
			if ref.Storage != GlobalRefType {
				vm.runtimeErrorf("cannot write through non-global var reference %d", varID)
				return
			}
			globalVar, ok := vm.globalVar(ref.Index)
			if !ok {
				vm.runtimeErrorf("failed to set global variable ID %d", varID)
				return
			}
			globalVar.Type = val.Type
			globalVar.Flags = val.Flags
			globalVar.Value = val.Value

		case OpGetLocal:
			localRef := UnpackVarRef(binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:]))
			vm.CurrentFrame.PC += 4
			val, ok := vm.resolveLocalVar(localRef.Index)
			if !ok {
				vm.runtimeErrorf("failed to resolve local slot %d", localRef.Index)
				return
			}
			liveRef, ok := vm.liveLocalRef(localRef.Index)
			if !ok {
				vm.runtimeErrorf("failed to resolve local ref %d", localRef.Index)
				return
			}
			vm.pushVarWithRef(val, liveRef)

		case OpSetLocal:
			localRef := UnpackVarRef(binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:]))
			vm.CurrentFrame.PC += 4
			valueRef := vm.popStackRef()
			val, ok := vm.resolveStackRef(block, valueRef)
			if !ok {
				vm.runtimeErrorf("failed to resolve stack ref %+v", valueRef)
				return
			}
			localVar, ok := vm.resolveLocalVar(localRef.Index)
			if !ok {
				vm.runtimeErrorf("failed to resolve local slot %d", localRef.Index)
				return
			}
			localVar.Type = val.Type
			localVar.Flags = val.Flags
			localVar.Value = val.Value

		case OpAdd, OpSub, OpMul, OpDiv, OpEQ, OpG, OpGE, OpL, OpLE:
			rightRef := vm.popStackRef()
			right, ok := vm.resolveStackRef(block, rightRef)
			if !ok {
				vm.runtimeErrorf("failed to resolve right stack ref %+v", rightRef)
				return
			}
			leftRef := vm.popStackRef()
			left, ok := vm.resolveStackRef(block, leftRef)
			if !ok {
				vm.runtimeErrorf("failed to resolve left stack ref %+v", leftRef)
				return
			}
			result := vm.applyBinaryOp(op, left, right)
			vm.pushTempVar(result)

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
			condRef := vm.popStackRef()
			condResult, ok := vm.resolveStackRef(block, condRef)
			if !ok {
				vm.runtimeErrorf("failed to resolve condition stack ref %+v", condRef)
				return
			}

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

			if !vm.ExecuteSyscallBlock(sysID, argCount) {
				return
			}

		case OpReturn:
			returnCount := block.Bytes[vm.CurrentFrame.PC]
			vm.CurrentFrame.PC++
			var returnValues [defaultReturnScratchSize]Var
			if int(returnCount) > len(returnValues) {
				panic("vm return scratch exhausted")
			}

			// Copy returned values out before reclaiming local/temp arena storage.
			retValues := returnValues[:returnCount]
			for i := int(returnCount) - 1; i >= 0; i-- {
				valueRef := vm.popStackRef()
				value, ok := vm.resolveStackRef(block, valueRef)
				if !ok {
					vm.runtimeErrorf("failed to resolve return stack ref %+v", valueRef)
					return
				}
				retValues[i] = *value
			}

			// Clear out the entire local variable space allocated to this frame pointer context
			vm.truncateStacks(vm.CurrentFrame.StackBase)
			vm.localTop = vm.CurrentFrame.LocalMark
			vm.tempTop = vm.CurrentFrame.TempMark

			// Push returned values back onto parent stack frame area
			for _, val := range retValues {
				vm.pushTempValue(val.Type, val.Flags&^VarFlagTemp, val.Value)
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

func (vm *VM) ExecuteSyscallBlock(sysID uint8, argCount uint8) bool {
	var syscallArgs [maxSyscallArgs]*Var
	if int(argCount) > len(syscallArgs) {
		panic("vm syscall scratch exhausted")
	}
	// Extract explicit argument items out of stack memory
	args := syscallArgs[:argCount]
	for i := int(argCount) - 1; i >= 0; i-- {
		argRef := vm.popStackRef()
		arg, ok := vm.resolveStackRef(vm.Blocks[vm.CurrentFrame.BlockID], argRef)
		if !ok {
			vm.runtimeErrorf("failed to resolve syscall arg ref %+v", argRef)
			return false
		}
		args[i] = arg
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
		vm.pushTempValue(VarTypeBool, VarFlagNone, isDone)

	case uint8(SystemCallSetLightOnOff):
		lightID := args[0]
		onOff := args[1]
		vm.SysCalls.SetLightOnOff(lightID.AsUint32(), onOff.AsUint32())

	case uint8(SystemCallIsLightOn):
		lightID := args[0]
		status := vm.SysCalls.IsLightOn(lightID.AsUint32())
		vm.pushTempValue(VarTypeBool, VarFlagNone, status)

	case uint8(SystemCallSetLightBrightness):
		lightID := args[0]
		brightness := args[1]
		vm.SysCalls.SetLightBrightness(lightID.AsUint32(), brightness.AsUint32())

	case uint8(SystemCallGetLightBrightness):
		lightID := args[0]
		brightness := vm.SysCalls.GetLightBrightness(lightID.AsUint32())
		vm.pushTempValue(VarTypeU32, VarFlagNone, brightness)

	case uint8(SystemCallSetLightColor):
		lightID := args[0]
		color := args[1]
		vm.SysCalls.SetLightColor(lightID.AsUint32(), color.AsUint32())

	case uint8(SystemCallGetLightColor):
		lightID := args[0]
		color := vm.SysCalls.GetLightColor(lightID.AsUint32())
		vm.pushTempValue(VarTypeU32, VarFlagNone, color)

	default:
		fmt.Printf("VM Warning: Triggered unregistered system block execution ID %d\n", sysID)
	}

	return true
}
