package vm

type Opcode uint8

const (
	OpReturn       Opcode = 0x00
	OpPushConstU8  Opcode = 0x01
	OpPushConstU16 Opcode = 0x02
	OpPushConstU32 Opcode = 0x03
	OpPushConstS8  Opcode = 0x04
	OpPushConstS16 Opcode = 0x05
	OpPushConstS32 Opcode = 0x06
	OpPushConstF32 Opcode = 0x07
	OpPushVar      Opcode = 0x08 // Reads a global predefined state variable [Type:8, Index:24]
	OpPopVar       Opcode = 0x09 // Writes to a global predefined state variable [Type:8, Index:24]
	OpIf           Opcode = 0x0A
	OpBinaryOp     Opcode = 0x0B
	OpSyscall      Opcode = 0x0C

	// Local Variable Storage Opcodes
	OpGetLocal Opcode = 0x0D // Pushes local var onto stack (Followed by 1 byte: Frame Offset)
	OpSetLocal Opcode = 0x0E // Pops value from stack into local var slot (Followed by 1 byte: Frame Offset)
)

type BinaryOpType uint8

const (
	OpAdd   BinaryOpType = 0
	OpSub   BinaryOpType = 1
	OpEqual BinaryOpType = 2
	OpG     BinaryOpType = 3
	OpGE    BinaryOpType = 4
	OpL     BinaryOpType = 5
	OpLE    BinaryOpType = 6
)
