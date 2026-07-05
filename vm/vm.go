package vm

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	TypeVariableU8  uint8 = 1
	TypeVariableU16 uint8 = 2
	TypeVariableU32 uint8 = 3
	TypeVariableS8  uint8 = 4
	TypeVariableS16 uint8 = 5
	TypeVariableS32 uint8 = 6
	TypeVariableF32 uint8 = 7
)

// VMBlock represents the loaded executable chunks inside the VM
type VMBlock struct {
	ID         uint32
	LocalCount uint32
	Bytes      []byte
}

// CallFrame tracks the context of an executing block
type CallFrame struct {
	BlockID      uint32
	PC           uint32
	FramePointer uint32 // Index in the data stack where this block's locals begin
}

type VM struct {
	Blocks map[uint32]VMBlock
	//GlobalState map[uint32]uint32 // Predefined shared external variables
	GlobalState VmGlobalStateInterface

	// Evaluation and Storage Stack
	DataStack []Value

	// Frame Call Stack
	CallStack    []CallFrame
	CurrentFrame CallFrame

	SysCalls VmSystemInterface
}

func NewVirtualMachine(systemCalls VmSystemInterface, globalState VmGlobalStateInterface) *VM {
	return &VM{
		Blocks:      make(map[uint32]VMBlock),
		GlobalState: globalState,
		DataStack:   make([]Value, 0, 256),
		CallStack:   make([]CallFrame, 0, 16),
		SysCalls:    systemCalls,
	}
}

func valueKindFromConstType(constType ConstType) ValueKind {
	switch constType {
	case ConstTypeS8, ConstTypeS16, ConstTypeS32:
		return ValueKindS32
	case ConstTypeF32:
		return ValueKindF32
	default:
		return ValueKindU32
	}
}

func valueKindFromStorageType(storageType uint8) ValueKind {
	switch storageType {
	case TypeVariableS8, TypeVariableS16, TypeVariableS32:
		return ValueKindS32
	case TypeVariableF32:
		return ValueKindF32
	default:
		return ValueKindU32
	}
}

func (vm *VM) pushValue(value Value) {
	vm.DataStack = append(vm.DataStack, value)
}

func (vm *VM) popValue() Value {
	topIdx := len(vm.DataStack) - 1
	value := vm.DataStack[topIdx]
	vm.DataStack = vm.DataStack[:topIdx]
	return value
}

func (vm *VM) pushStoredValue(storageType uint8, bits uint32) {
	vm.pushValue(Value{Kind: valueKindFromStorageType(storageType), Bits: bits})
}

func (vm *VM) binaryResultKind(left, right Value) ValueKind {
	if left.Kind == ValueKindF32 || right.Kind == ValueKindF32 {
		return ValueKindF32
	}
	if left.Kind == ValueKindS32 || right.Kind == ValueKindS32 {
		return ValueKindS32
	}
	return ValueKindU32
}

func (vm *VM) comparisonResult(ok bool) Value {
	if ok {
		return Value{Kind: ValueKindU32, Bits: 1}
	}
	return Value{Kind: ValueKindU32, Bits: 0}
}

func (vm *VM) applyBinaryOp(opType BinaryOpType, left, right Value) Value {
	resultKind := vm.binaryResultKind(left, right)

	switch opType {
	case OpAdd:
		switch resultKind {
		case ValueKindF32:
			return Value{Kind: ValueKindF32, Bits: math.Float32bits(left.AsFloat32() + right.AsFloat32())}
		case ValueKindS32:
			return Value{Kind: ValueKindS32, Bits: uint32(left.AsInt32() + right.AsInt32())}
		default:
			return Value{Kind: ValueKindU32, Bits: left.AsUint32() + right.AsUint32()}
		}
	case OpSub:
		switch resultKind {
		case ValueKindF32:
			return Value{Kind: ValueKindF32, Bits: math.Float32bits(left.AsFloat32() - right.AsFloat32())}
		case ValueKindS32:
			return Value{Kind: ValueKindS32, Bits: uint32(left.AsInt32() - right.AsInt32())}
		default:
			return Value{Kind: ValueKindU32, Bits: left.AsUint32() - right.AsUint32()}
		}
	case OpMul:
		switch resultKind {
		case ValueKindF32:
			return Value{Kind: ValueKindF32, Bits: math.Float32bits(left.AsFloat32() * right.AsFloat32())}
		case ValueKindS32:
			return Value{Kind: ValueKindS32, Bits: uint32(left.AsInt32() * right.AsInt32())}
		default:
			return Value{Kind: ValueKindU32, Bits: left.AsUint32() * right.AsUint32()}
		}
	case OpDiv:
		switch resultKind {
		case ValueKindF32:
			leftValue := left.AsFloat32()
			rightValue := right.AsFloat32()
			if rightValue == 0 {
				if leftValue < 0 {
					return Value{Kind: ValueKindF32, Bits: math.Float32bits(float32(math.Inf(-1)))}
				}
				return Value{Kind: ValueKindF32, Bits: math.Float32bits(float32(math.Inf(1)))}
			}
			return Value{Kind: ValueKindF32, Bits: math.Float32bits(leftValue / rightValue)}
		case ValueKindS32:
			leftValue := left.AsInt32()
			rightValue := right.AsInt32()
			if rightValue == 0 {
				if leftValue < 0 {
					return Value{Kind: ValueKindS32, Bits: uint32(1 << 31)}
				}
				return Value{Kind: ValueKindS32, Bits: uint32(int32(1<<31 - 1))}
			}
			return Value{Kind: ValueKindS32, Bits: uint32(leftValue / rightValue)}
		default:
			leftValue := left.AsUint32()
			rightValue := right.AsUint32()
			if rightValue == 0 {
				return Value{Kind: ValueKindU32, Bits: math.MaxUint32}
			}
			return Value{Kind: ValueKindU32, Bits: leftValue / rightValue}
		}
	case OpEQ:
		switch resultKind {
		case ValueKindF32:
			return vm.comparisonResult(left.AsFloat32() == right.AsFloat32())
		case ValueKindS32:
			return vm.comparisonResult(left.AsInt32() == right.AsInt32())
		default:
			return vm.comparisonResult(left.AsUint32() == right.AsUint32())
		}
	case OpG:
		switch resultKind {
		case ValueKindF32:
			return vm.comparisonResult(left.AsFloat32() > right.AsFloat32())
		case ValueKindS32:
			return vm.comparisonResult(left.AsInt32() > right.AsInt32())
		default:
			return vm.comparisonResult(left.AsUint32() > right.AsUint32())
		}
	case OpGE:
		switch resultKind {
		case ValueKindF32:
			return vm.comparisonResult(left.AsFloat32() >= right.AsFloat32())
		case ValueKindS32:
			return vm.comparisonResult(left.AsInt32() >= right.AsInt32())
		default:
			return vm.comparisonResult(left.AsUint32() >= right.AsUint32())
		}
	case OpL:
		switch resultKind {
		case ValueKindF32:
			return vm.comparisonResult(left.AsFloat32() < right.AsFloat32())
		case ValueKindS32:
			return vm.comparisonResult(left.AsInt32() < right.AsInt32())
		default:
			return vm.comparisonResult(left.AsUint32() < right.AsUint32())
		}
	case OpLE:
		switch resultKind {
		case ValueKindF32:
			return vm.comparisonResult(left.AsFloat32() <= right.AsFloat32())
		case ValueKindS32:
			return vm.comparisonResult(left.AsInt32() <= right.AsInt32())
		default:
			return vm.comparisonResult(left.AsUint32() <= right.AsUint32())
		}
	default:
		return Value{Kind: ValueKindU32, Bits: 0}
	}
}

func (vm *VM) ExecuteBlock(blockID uint32) {
	vm.CurrentFrame = vm.allocateFrame(blockID)
	vm.executeCurrentFrame()
}

func (vm *VM) allocateFrame(blockID uint32) CallFrame {
	// Setup the initial root call frame
	block := vm.Blocks[blockID]
	fp := uint32(len(vm.DataStack))

	// Allocate blank padding spaces on the data stack for local variables
	for i := uint32(0); i < block.LocalCount; i++ {
		vm.pushValue(Value{Kind: ValueKindU32, Bits: 0})
	}

	return CallFrame{BlockID: blockID, PC: 0, FramePointer: fp}
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
		case OpPushConst:
			constType := ConstType(block.Bytes[vm.CurrentFrame.PC])
			vm.CurrentFrame.PC++

			var val uint32
			switch constType {
			case ConstTypeU8:
				val = uint32(block.Bytes[vm.CurrentFrame.PC])
				vm.CurrentFrame.PC++
			case ConstTypeU16:
				val = uint32(binary.LittleEndian.Uint16(block.Bytes[vm.CurrentFrame.PC:]))
				vm.CurrentFrame.PC += 2
			case ConstTypeU32:
				val = binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
				vm.CurrentFrame.PC += 4
			case ConstTypeS8:
				val = uint32(int32(int8(block.Bytes[vm.CurrentFrame.PC])))
				vm.CurrentFrame.PC++
			case ConstTypeS16:
				val = uint32(int32(int16(binary.LittleEndian.Uint16(block.Bytes[vm.CurrentFrame.PC:]))))
				vm.CurrentFrame.PC += 2
			case ConstTypeS32, ConstTypeF32:
				val = binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
				vm.CurrentFrame.PC += 4
			default:
				fmt.Printf("VM Error: Unknown ConstType %d\n", constType)
				return
			}
			vm.pushValue(Value{Kind: valueKindFromConstType(constType), Bits: val})

		case OpPushVar:
			varID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			globalVar, ok := vm.GlobalState.GetGlobalVar(NewID(varID))
			if !ok {
				fmt.Printf("VM Error: Global variable ID %d not found\n", varID)
				return
			}
			vm.pushStoredValue(NewID(varID).Type, globalVar)

		case OpPopVar:
			varID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			val := vm.popValue()
			if !vm.GlobalState.SetGlobalVar(NewID(varID), val.Bits) {
				fmt.Printf("VM Error: Failed to set global variable ID %d\n", varID)
				return
			}

		case OpGetLocal:
			localID := NewID(binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:]))
			vm.CurrentFrame.PC += 4
			val := vm.DataStack[vm.CurrentFrame.FramePointer+localID.Idx]
			vm.pushValue(val)

		case OpSetLocal:
			localID := NewID(binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:]))
			vm.CurrentFrame.PC += 4
			val := vm.popValue()
			vm.DataStack[vm.CurrentFrame.FramePointer+localID.Idx] = val

		case OpBinaryOp:
			opType := BinaryOpType(block.Bytes[vm.CurrentFrame.PC])
			vm.CurrentFrame.PC++

			right := vm.popValue()
			left := vm.popValue()
			vm.pushValue(vm.applyBinaryOp(opType, left, right))

		case OpIf:
			condBlockID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			trueBlockID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			falseBlockID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4

			// 1. Save our position and branch out to solve the condition block
			vm.CallStack = append(vm.CallStack, vm.CurrentFrame)

			// 2. Prepare and run the condition block
			vm.CurrentFrame = vm.allocateFrame(condBlockID)

			// Execute sub-loop synchronously until the condition block hits OpReturn
			vm.executeSubLoop()

			// 3. Capture the condition boolean left on the top of the stack
			condResult := vm.popValue()

			// 4. Choose which structural body execution pipeline block to execute next
			targetBlockID := falseBlockID
			if condResult.Bits != 0 {
				targetBlockID = trueBlockID
			}

			vm.CallStack = append(vm.CallStack, vm.CurrentFrame)
			vm.CurrentFrame = vm.allocateFrame(targetBlockID)
			vm.executeSubLoop()

		case OpSyscall:
			sysID := block.Bytes[vm.CurrentFrame.PC]
			vm.CurrentFrame.PC++
			argCount := block.Bytes[vm.CurrentFrame.PC]
			vm.CurrentFrame.PC++

			vm.ExecuteSyscallBlock(sysID, argCount)

		case OpReturn:
			returnCount := block.Bytes[vm.CurrentFrame.PC]
			vm.CurrentFrame.PC++

			// Extract returned values sitting on the very top of the stack frame
			retValues := make([]Value, returnCount)
			for i := int(returnCount) - 1; i >= 0; i-- {
				retValues[i] = vm.popValue()
			}

			// Clear out the entire local variable space allocated to this frame pointer context
			vm.DataStack = vm.DataStack[:vm.CurrentFrame.FramePointer]

			// Push returned values back onto parent stack frame area
			for _, val := range retValues {
				vm.pushValue(val)
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
	// Extract explicit argument items out of stack memory
	args := make([]Value, argCount)
	for i := int(argCount) - 1; i >= 0; i-- {
		args[i] = vm.popValue()
	}

	switch sysID {
	case uint8(SystemCallDrawBackground):
		//vm.appendDrawLog("DrawBackground(Image: %d)", args[0])
		vm.SysCalls.DrawBackground(args[0].AsUint32())

	case uint8(SystemCallDrawSprite):
		//vm.appendDrawLog("DrawSprite(Sprite: %d, X: %d, Y: %d)", args[0], args[1], args[2])
		vm.SysCalls.DrawSprite(args[0].AsUint32(), args[1].AsUint32(), args[2].AsUint32())

	case uint8(SystemCallDrawText):
		//vm.appendDrawLog("DrawText(Font: %d, Text: %d, X: %d, Y: %d, Color: %d)", args[0], args[1], args[2], args[3], args[4])
		vm.SysCalls.DrawText(args[0].AsUint32(), args[1].AsUint32(), args[2].AsUint32(), args[3].AsUint32(), args[4].AsUint32())

	case uint8(SystemCallDrawVar):
		//vm.appendDrawLog("DrawVar(Font: %d, Var: %d, X: %d, Y: %d, Color: %d)", args[0], args[1], args[2], args[3], args[4])
		vm.SysCalls.DrawVar(args[0].AsUint32(), args[1].AsUint32(), args[2].AsUint32(), args[3].AsUint32(), args[4].AsUint32())

	case uint8(SystemCallStartTimer):
		timerID := args[0]
		duration := args[1]
		vm.SysCalls.StartTimer(timerID.AsUint32(), duration.AsUint32())

	case uint8(SystemCallStopTimer):
		timerID := args[0]
		vm.SysCalls.StopTimer(timerID.AsUint32())

	case uint8(SystemCallGetTimer):
		timerID := args[0]
		timerValue := vm.SysCalls.GetTimer(timerID.AsUint32())
		vm.pushValue(Value{Kind: ValueKindU32, Bits: timerValue})

	case uint8(SystemCallSetLightOnOff):
		lightID := args[0]
		onOff := args[1]
		vm.SysCalls.SetLightOnOff(lightID.AsUint32(), onOff.AsUint32())

	case uint8(SystemCallIsLightOn):
		lightID := args[0]
		status := vm.SysCalls.IsLightOn(lightID.AsUint32())
		vm.pushValue(Value{Kind: ValueKindU32, Bits: status})

	case uint8(SystemCallSetLightBrightness):
		lightID := args[0]
		brightness := args[1]
		vm.SysCalls.SetLightBrightness(lightID.AsUint32(), brightness.AsUint32())

	case uint8(SystemCallGetLightBrightness):
		lightID := args[0]
		brightness := vm.SysCalls.GetLightBrightness(lightID.AsUint32())
		vm.pushValue(Value{Kind: ValueKindU32, Bits: brightness})

	case uint8(SystemCallSetLightColor):
		lightID := args[0]
		color := args[1]
		vm.SysCalls.SetLightColor(lightID.AsUint32(), color.AsUint32())

	case uint8(SystemCallGetLightColor):
		lightID := args[0]
		color := vm.SysCalls.GetLightColor(lightID.AsUint32())
		vm.pushValue(Value{Kind: ValueKindU32, Bits: color})

	default:
		fmt.Printf("VM Warning: Triggered unregistered system block execution ID %d\n", sysID)
	}
}
