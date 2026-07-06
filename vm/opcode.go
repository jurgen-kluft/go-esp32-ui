package vm

import (
	"go/token"
)

type Opcode uint8

const (
	OpReturn  Opcode = 0x00
	OpPushVar Opcode = 0x08 // Reads a var reference [StorageClass:8, Index:24]
	OpPopVar  Opcode = 0x09 // Writes to a var reference [StorageClass:8, Index:24]
	OpIf      Opcode = 0x0A
	OpSyscall Opcode = 0x0C

	// Local Variable Storage Opcodes
	OpGetLocal Opcode = 0x0D // Pushes local var onto stack (Followed by packed [StorageClass:8, Index:24])
	OpSetLocal Opcode = 0x0E // Pops value from stack into local var slot (Followed by packed [StorageClass:8, Index:24])

	// Arithmetic Opcodes
	OpAdd Opcode = 0x0F
	OpSub Opcode = 0x10
	OpMul Opcode = 0x11
	OpDiv Opcode = 0x12
	OpEQ  Opcode = 0x13
	OpG   Opcode = 0x14
	OpGE  Opcode = 0x15
	OpL   Opcode = 0x16
	OpLE  Opcode = 0x17
)

func tokenToOpcode(t token.Token) (Opcode, bool) {
	switch t {
	case token.ADD:
		return OpAdd, true
	case token.SUB:
		return OpSub, true
	case token.MUL:
		return OpMul, true
	case token.QUO:
		return OpDiv, true
	case token.EQL:
		return OpEQ, true
	case token.GTR:
		return OpG, true
	case token.GEQ:
		return OpGE, true
	case token.LSS:
		return OpL, true
	case token.LEQ:
		return OpLE, true
	}
	return 0, false
}
