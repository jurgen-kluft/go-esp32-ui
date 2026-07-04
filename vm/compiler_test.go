package vm

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestCompileLocalOperandsUsePackedIDs(t *testing.T) {
	block := compileBlockForTest(t, nil, "x := 42\nreturn x")
	want := (ID{Type: LocalIDType, Idx: 0}).Pack()

	if len(block.Bytes) < 13 {
		t.Fatalf("bytecode too short: %v", block.Bytes)
	}

	if block.Bytes[3] != byte(OpSetLocal) {
		t.Fatalf("expected OpSetLocal at offset 3, got %d", block.Bytes[3])
	}
	if got := binary.LittleEndian.Uint32(block.Bytes[4:8]); got != want {
		t.Fatalf("OpSetLocal operand mismatch: got %#08x want %#08x", got, want)
	}

	if block.Bytes[8] != byte(OpGetLocal) {
		t.Fatalf("expected OpGetLocal at offset 8, got %d", block.Bytes[8])
	}
	if got := binary.LittleEndian.Uint32(block.Bytes[9:13]); got != want {
		t.Fatalf("OpGetLocal operand mismatch: got %#08x want %#08x", got, want)
	}
}

func TestCompileIntLiteralUsesTaggedConstEncoding(t *testing.T) {
	testCases := []struct {
		name string
		src  string
		want []byte
	}{
		{
			name: "u8",
			src:  "return 42",
			want: []byte{byte(OpPushConst), byte(ConstTypeU8), 42, byte(OpReturn), 1},
		},
		{
			name: "u16",
			src:  "return 256",
			want: []byte{byte(OpPushConst), byte(ConstTypeU16), 0x00, 0x01, byte(OpReturn), 1},
		},
		{
			name: "u32",
			src:  "return 65536",
			want: []byte{byte(OpPushConst), byte(ConstTypeU32), 0x00, 0x00, 0x01, 0x00, byte(OpReturn), 1},
		},
		{
			name: "s8",
			src:  "return -1",
			want: []byte{byte(OpPushConst), byte(ConstTypeS8), 0xFF, byte(OpReturn), 1},
		},
		{
			name: "s16",
			src:  "return -129",
			want: []byte{byte(OpPushConst), byte(ConstTypeS16), 0x7F, 0xFF, byte(OpReturn), 1},
		},
		{
			name: "s32",
			src:  "return -32769",
			want: []byte{byte(OpPushConst), byte(ConstTypeS32), 0xFF, 0x7F, 0xFF, 0xFF, byte(OpReturn), 1},
		},
		{
			name: "f32",
			src:  "return 1.5",
			want: []byte{byte(OpPushConst), byte(ConstTypeF32), 0x00, 0x00, 0xC0, 0x3F, byte(OpReturn), 1},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			block := compileBlockForTest(t, nil, testCase.src)

			if len(block.Bytes) != len(testCase.want) {
				t.Fatalf("unexpected bytecode length: got %d want %d (%v)", len(block.Bytes), len(testCase.want), block.Bytes)
			}
			for i, wantByte := range testCase.want {
				if block.Bytes[i] != wantByte {
					t.Fatalf("unexpected byte at offset %d: got %d want %d (%v)", i, block.Bytes[i], wantByte, block.Bytes)
				}
			}
		})
	}
}

func TestCompileFloatAndSignedLiteralsExecute(t *testing.T) {
	testCases := []struct {
		name string
		src  string
		want uint32
	}{
		{name: "u16", src: "return 256", want: 256},
		{name: "s8", src: "return -1", want: 0xFFFFFFFF},
		{name: "s16", src: "return -129", want: 0xFFFFFF7F},
		{name: "s32", src: "return -32769", want: 0xFFFF7FFF},
		{name: "f32", src: "return 1.5", want: math.Float32bits(1.5)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			block := compileBlockForTest(t, nil, testCase.src)

			sys := NewTestSystemInterface()
			vm := NewVirtualMachine(sys)
			vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes}
			vm.ExecuteBlock(block.ID)

			if len(vm.DataStack) != 1 || vm.DataStack[0].Bits != testCase.want {
				t.Fatalf("unexpected stack after execution: %v want %d", vm.DataStack, testCase.want)
			}
		})
	}
}

func TestCompileUnaryArithmeticExecute(t *testing.T) {
	testCases := []struct {
		name string
		src  string
		want uint32
	}{
		{name: "unary plus literal", src: "return +42", want: 42},
		{name: "unary minus local", src: "x := 5\nreturn -x", want: 0xFFFFFFFB},
		{name: "unary minus expression", src: "return -(1 + 2)", want: 0xFFFFFFFD},
		{name: "nested unary plus expression", src: "return +(1 + 2)", want: 3},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			block := compileBlockForTest(t, nil, testCase.src)

			sys := NewTestSystemInterface()
			vm := NewVirtualMachine(sys)
			vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes}
			vm.ExecuteBlock(block.ID)

			if len(vm.DataStack) != 1 || vm.DataStack[0].Bits != testCase.want {
				t.Fatalf("unexpected stack after execution: %v want %d", vm.DataStack, testCase.want)
			}
		})
	}
}

func TestUnsupportedBinaryOperatorFailsCompilation(t *testing.T) {
	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(nil, systemInterface)
	block := compiler.AllocateBlock()
	err := compiler.CompileBlock(block, parseStatements(t, "return 9 / 4"))
	if err == nil {
		t.Fatal("expected compile error for unsupported operator")
	}

	want := "unsupported binary operator: /"
	if err.Error() != want {
		t.Fatalf("unexpected compile error: got %q want %q", err.Error(), want)
	}
}

func TestBinaryExpressionPropagatesOperandErrors(t *testing.T) {
	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(nil, systemInterface)
	block := compiler.AllocateBlock()
	err := compiler.CompileBlock(block, parseStatements(t, "return missing + 1"))
	if err == nil {
		t.Fatal("expected compile error for missing operand")
	}

	want := fmt.Sprintf("compile error: variable '%s' undefined", "missing")
	if err.Error() != want {
		t.Fatalf("unexpected compile error: got %q want %q", err.Error(), want)
	}
}

func TestCompilerRegistersDesignSyscalls(t *testing.T) {
	systemInterface := NewCompilerSystemCalls()

	want := map[string]uint8{
		"DrawBackground":     uint8(SystemCallDrawBackground),
		"DrawSprite":         uint8(SystemCallDrawSprite),
		"DrawText":           uint8(SystemCallDrawText),
		"DrawVar":            uint8(SystemCallDrawVar),
		"StartTimer":         uint8(SystemCallStartTimer),
		"StopTimer":          uint8(SystemCallStopTimer),
		"GetTimer":           uint8(SystemCallGetTimer),
		"SetLightOnOff":      uint8(SystemCallSetLightOnOff),
		"IsLightOn":          uint8(SystemCallIsLightOn),
		"SetLightBrightness": uint8(SystemCallSetLightBrightness),
		"GetLightBrightness": uint8(SystemCallGetLightBrightness),
		"SetLightColor":      uint8(SystemCallSetLightColor),
		"GetLightColor":      uint8(SystemCallGetLightColor),
	}

	for name, id := range want {
		got, ok := systemInterface.RegisterSystemCall(name)
		if !ok {
			t.Fatalf("missing syscall registration for %s", name)
		}
		if got != id {
			t.Fatalf("wrong syscall id for %s: got %d want %d", name, got, id)
		}
	}
}
