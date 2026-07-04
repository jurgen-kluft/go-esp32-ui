package vm

import (
	"encoding/binary"
	"fmt"
)

// VMBlock represents the loaded executable chunks inside the VM
type VMBlock struct {
	ID         uint32
	LocalCount uint8
	Bytes      []byte
}

// CallFrame tracks the context of an executing block
type CallFrame struct {
	BlockID      uint32
	PC           uint32
	FramePointer uint32 // Index in the data stack where this block's locals begin
}

type VirtualMachine struct {
	Blocks      map[uint32]VMBlock
	GlobalState map[uint32]uint32 // Predefined shared external variables

	// Evaluation and Storage Stack
	DataStack []uint32

	// Frame Call Stack
	CallStack    []CallFrame
	CurrentFrame CallFrame

	// Simulated Hardware State
	LightsOn map[uint32]uint32
}

func NewVirtualMachine() *VirtualMachine {
	return &VirtualMachine{
		Blocks:      make(map[uint32]VMBlock),
		GlobalState: make(map[uint32]uint32),
		DataStack:   make([]uint32, 0, 256),
		CallStack:   make([]CallFrame, 0, 16),
		LightsOn:    make(map[uint32]uint32),
	}
}

func (vm *VirtualMachine) ExecuteBlock(blockID uint32) {
	// Setup the initial root call frame
	block := vm.Blocks[blockID]
	fp := uint32(len(vm.DataStack))

	// Allocate blank padding spaces on the data stack for local variables
	for i := uint8(0); i < block.LocalCount; i++ {
		vm.DataStack = append(vm.DataStack, 0)
	}

	vm.CurrentFrame = CallFrame{BlockID: blockID, PC: 0, FramePointer: fp}

	for {
		block = vm.Blocks[vm.CurrentFrame.BlockID]
		if vm.CurrentFrame.PC >= uint32(len(block.Bytes)) {
			fmt.Println("VM Error: Program Counter ran out of bounds")
			return
		}

		op := Opcode(block.Bytes[vm.CurrentFrame.PC])
		vm.CurrentFrame.PC++

		switch op {
		case OpPushConstU8:
			val := uint32(block.Bytes[vm.CurrentFrame.PC])
			vm.CurrentFrame.PC++
			vm.DataStack = append(vm.DataStack, val)

		case OpPushConstU32:
			val := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			vm.DataStack = append(vm.DataStack, val)

		case OpPushVar:
			varID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			vm.DataStack = append(vm.DataStack, vm.GlobalState[varID])

		case OpPopVar:
			varID := binary.LittleEndian.Uint32(block.Bytes[vm.CurrentFrame.PC:])
			vm.CurrentFrame.PC += 4
			topIdx := len(vm.DataStack) - 1
			val := vm.DataStack[topIdx]
			vm.DataStack = vm.DataStack[:topIdx] // Pop
			vm.GlobalState[varID] = val

		case OpGetLocal:
			localIdx := uint32(block.Bytes[vm.CurrentFrame.PC])
			vm.CurrentFrame.PC++
			val := vm.DataStack[vm.CurrentFrame.FramePointer+localIdx]
			vm.DataStack = append(vm.DataStack, val)

		case OpSetLocal:
			localIdx := uint32(block.Bytes[vm.CurrentFrame.PC])
			vm.CurrentFrame.PC++
			topIdx := len(vm.DataStack) - 1
			val := vm.DataStack[topIdx]
			vm.DataStack = vm.DataStack[:topIdx] // Pop
			vm.DataStack[vm.CurrentFrame.FramePointer+localIdx] = val

		case OpBinaryOp:
			opType := BinaryOpType(block.Bytes[vm.CurrentFrame.PC])
			vm.CurrentFrame.PC++

			rIdx := len(vm.DataStack) - 1
			lIdx := len(vm.DataStack) - 2
			left := vm.DataStack[lIdx]
			right := vm.DataStack[rIdx]
			vm.DataStack = vm.DataStack[:lIdx] // Pop both

			var result uint32
			switch opType {
			case OpAdd:
				result = left + right
			case OpSub:
				result = left - right
			case OpEqual:
				if left == right {
					result = 1
				} else {
					result = 0
				}
			case OpG:
				if left > right {
					result = 1
				} else {
					result = 0
				}
			case OpGE:
				if left >= right {
					result = 1
				} else {
					result = 0
				}
			case OpL:
				if left < right {
					result = 1
				} else {
					result = 0
				}
			case OpLE:
				if left <= right {
					result = 1
				} else {
					result = 0
				}
			}
			vm.DataStack = append(vm.DataStack, result)

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
			cBlock := vm.Blocks[condBlockID]
			cfp := uint32(len(vm.DataStack))
			for i := uint8(0); i < cBlock.LocalCount; i++ {
				vm.DataStack = append(vm.DataStack, 0)
			}

			vm.CurrentFrame = CallFrame{BlockID: condBlockID, PC: 0, FramePointer: cfp}

			// Execute sub-loop synchronously until the condition block hits OpReturn
			vm.executeSubLoop()

			// 3. Capture the condition boolean left on the top of the stack
			topIdx := len(vm.DataStack) - 1
			condResult := vm.DataStack[topIdx]
			vm.DataStack = vm.DataStack[:topIdx] // Pop result

			// 4. Choose which structural body execution pipeline block to execute next
			targetBlockID := falseBlockID
			if condResult != 0 {
				targetBlockID = trueBlockID
			}

			tBlock := vm.Blocks[targetBlockID]
			tfp := uint32(len(vm.DataStack))
			for i := uint8(0); i < tBlock.LocalCount; i++ {
				vm.DataStack = append(vm.DataStack, 0)
			}

			vm.CurrentFrame = CallFrame{BlockID: targetBlockID, PC: 0, FramePointer: tfp}
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
			retValues := make([]uint32, returnCount)
			for i := int(returnCount) - 1; i >= 0; i-- {
				topIdx := len(vm.DataStack) - 1
				retValues[i] = vm.DataStack[topIdx]
				vm.DataStack = vm.DataStack[:topIdx]
			}

			// Clear out the entire local variable space allocated to this frame pointer context
			vm.DataStack = vm.DataStack[:vm.CurrentFrame.FramePointer]

			// Push returned values back onto parent stack frame area
			for _, val := range retValues {
				vm.DataStack = append(vm.DataStack, val)
			}
			return
		}
	}
}

// Inline helper loop to safely step over branching frames nested in execution structures
func (vm *VirtualMachine) executeSubLoop() {
	frameStartDepth := len(vm.CallStack)
	vm.ExecuteBlock(vm.CurrentFrame.BlockID)

	// Pop frame safely when execution winds down back to standard context levels
	if len(vm.CallStack) > frameStartDepth {
		vm.CurrentFrame = vm.CallStack[len(vm.CallStack)-1]
		vm.CallStack = vm.CallStack[:len(vm.CallStack)-1]
	}
}

func (vm *VirtualMachine) ExecuteSyscallBlock(sysID uint8, argCount uint8) {
	// Extract explicit argument items out of stack memory
	args := make([]uint32, argCount)
	for i := int(argCount) - 1; i >= 0; i-- {
		topIdx := len(vm.DataStack) - 1
		args[i] = vm.DataStack[topIdx]
		vm.DataStack = vm.DataStack[:topIdx]
	}

	switch sysID {
	case 1: // IsLightOn(lightId) -> Returns 1 or 0
		lightID := args[0]
		status := vm.LightsOn[lightID]

		// Push returned data back onto evaluation pipeline
		vm.DataStack = append(vm.DataStack, status)
		fmt.Printf("[Syscall] IsLightOn(ID: %d) -> Returns %d\n", lightID, status)

	case 2: // SetLightOnOff(lightId, onOff) -> Returns nothing
		lightID := args[0]
		onOff := args[1]
		vm.LightsOn[lightID] = onOff
		fmt.Printf("[Syscall] SetLightOnOff(ID: %d, State: %d)\n", lightID, onOff)

	case 3: // DrawSprite(spriteId, x, y)
		fmt.Printf("[Syscall] DrawSprite(Sprite: %d, X: %d, Y: %d)\n", args[0], args[1], args[2])

	default:
		fmt.Printf("VM Warning: Triggered unregistered system block execution ID %d\n", sysID)
	}
}
