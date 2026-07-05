package vm

const (
	LocalIDType uint8  = 0
	idIndexMask uint32 = 0x00FFFFFF
	idTypeShift uint32 = 24
)

type Opcode uint8

type ConstType uint8

const (
	ConstTypeU8 ConstType = iota
	ConstTypeU16
	ConstTypeU32
	ConstTypeS8
	ConstTypeS16
	ConstTypeS32
	ConstTypeF32
)

const (
	OpReturn    Opcode = 0x00
	OpPushConst Opcode = 0x01 // Reads a ConstType byte plus payload.
	OpPushVar   Opcode = 0x08 // Reads a global predefined state variable [Type:8, Index:24]
	OpPopVar    Opcode = 0x09 // Writes to a global predefined state variable [Type:8, Index:24]
	OpIf        Opcode = 0x0A
	OpBinaryOp  Opcode = 0x0B
	OpSyscall   Opcode = 0x0C

	// Local Variable Storage Opcodes
	OpGetLocal Opcode = 0x0D // Pushes local var onto stack (Followed by packed [Type:8, Index:24])
	OpSetLocal Opcode = 0x0E // Pops value from stack into local var slot (Followed by packed [Type:8, Index:24])
)

func (id ID) Pack() uint32 {
	return (uint32(id.Type) << idTypeShift) | (id.Idx & idIndexMask)
}

func NewID(raw uint32) ID {
	return ID{
		Type: uint8(raw >> idTypeShift),
		Idx:  raw & idIndexMask,
	}
}

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
