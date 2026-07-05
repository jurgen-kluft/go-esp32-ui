package vm

type Opcode uint8

const (
	OpReturn   Opcode = 0x00
	OpPushVar  Opcode = 0x08 // Reads a var reference [StorageClass:8, Index:24]
	OpPopVar   Opcode = 0x09 // Writes to a var reference [StorageClass:8, Index:24]
	OpIf       Opcode = 0x0A
	OpBinaryOp Opcode = 0x0B
	OpSyscall  Opcode = 0x0C

	// Local Variable Storage Opcodes
	OpGetLocal Opcode = 0x0D // Pushes local var onto stack (Followed by packed [StorageClass:8, Index:24])
	OpSetLocal Opcode = 0x0E // Pops value from stack into local var slot (Followed by packed [StorageClass:8, Index:24])
)

type BinaryOpType uint8

const (
	OpAdd BinaryOpType = 0
	OpSub BinaryOpType = 1
	OpMul BinaryOpType = 2
	OpDiv BinaryOpType = 3
	OpEQ  BinaryOpType = 4
	OpG   BinaryOpType = 5
	OpGE  BinaryOpType = 6
	OpL   BinaryOpType = 7
	OpLE  BinaryOpType = 8
)
