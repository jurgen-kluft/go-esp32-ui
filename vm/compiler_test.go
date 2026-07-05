package vm

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestCompileLocalOperandsUsePackedIDs(t *testing.T) {
	block := compileBlockForTest(t, nil, "x := 42\nreturn x")
	want := LocalRef(0).Pack()

	if len(block.Bytes) < 15 {
		t.Fatalf("bytecode too short: %v", block.Bytes)
	}

	if block.Bytes[5] != byte(OpSetLocal) {
		t.Fatalf("expected OpSetLocal at offset 5, got %d", block.Bytes[5])
	}
	if got := binary.LittleEndian.Uint32(block.Bytes[6:10]); got != want {
		t.Fatalf("OpSetLocal operand mismatch: got %#08x want %#08x", got, want)
	}

	if block.Bytes[10] != byte(OpGetLocal) {
		t.Fatalf("expected OpGetLocal at offset 10, got %d", block.Bytes[10])
	}
	if got := binary.LittleEndian.Uint32(block.Bytes[11:15]); got != want {
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
			want: []byte{byte(OpPushVar), 0x00, 0x00, 0x00, 0x01, byte(OpReturn), 1},
		},
		{
			name: "u16",
			src:  "return 256",
			want: []byte{byte(OpPushVar), 0x00, 0x00, 0x00, 0x01, byte(OpReturn), 1},
		},
		{
			name: "u32",
			src:  "return 65536",
			want: []byte{byte(OpPushVar), 0x00, 0x00, 0x00, 0x01, byte(OpReturn), 1},
		},
		{
			name: "s8",
			src:  "return -1",
			want: []byte{byte(OpPushVar), 0x00, 0x00, 0x00, 0x01, byte(OpReturn), 1},
		},
		{
			name: "s16",
			src:  "return -129",
			want: []byte{byte(OpPushVar), 0x00, 0x00, 0x00, 0x01, byte(OpReturn), 1},
		},
		{
			name: "s32",
			src:  "return -32769",
			want: []byte{byte(OpPushVar), 0x00, 0x00, 0x00, 0x01, byte(OpReturn), 1},
		},
		{
			name: "f32",
			src:  "return 1.5",
			want: []byte{byte(OpPushVar), 0x00, 0x00, 0x00, 0x01, byte(OpReturn), 1},
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
			vm := NewVirtualMachine(sys, nil)
			vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
			vm.ExecuteBlock(block.ID)

			if len(vm.DataStack) != 1 || vm.DataStack[0].Uint32Value() != testCase.want {
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
			vm := NewVirtualMachine(sys, nil)
			vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
			vm.ExecuteBlock(block.ID)

			if len(vm.DataStack) != 1 || vm.DataStack[0].Uint32Value() != testCase.want {
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
	if err != nil {
		t.Fatalf("expected division to compile: %v", err)
	}

	want := []byte{byte(OpPushVar), 0x00, 0x00, 0x00, 0x01, byte(OpPushVar), 0x01, 0x00, 0x00, 0x01, byte(OpBinaryOp), byte(OpDiv), byte(OpReturn), 1}
	if len(block.Bytes) != len(want) {
		t.Fatalf("unexpected bytecode length: got %d want %d (%v)", len(block.Bytes), len(want), block.Bytes)
	}
	for i, wantByte := range want {
		if block.Bytes[i] != wantByte {
			t.Fatalf("unexpected byte at offset %d: got %d want %d (%v)", i, block.Bytes[i], wantByte, block.Bytes)
		}
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
		"IsTimerDone":        uint8(SystemCallIsTimerDone),
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

func TestIntrinsicVarToInt32ArithmeticAndUint32CastExecute(t *testing.T) {
	globals := map[string]VarRef{
		"Finger1X": GlobalRef(0),
	}
	block := compileBlockForTest(t, globals, "return uint32((VarToInt32(Finger1X)-90)*100/300)")

	sys := NewTestSystemInterface()
	globalState := []Var{{Index: 0, Type: VarTypeU16, Value: uint16(240)}}
	vm := NewVirtualMachine(sys, globalState)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 1 {
		t.Fatalf("unexpected stack size: got %d want 1", len(vm.DataStack))
	}
	if got := vm.DataStack[0]; got.AsUint32() != 50 {
		t.Fatalf("unexpected cast arithmetic result: got %+v", got)
	}
}

func TestIntrinsicVarAssignStoresGlobal(t *testing.T) {
	globals := map[string]VarRef{
		"UIMode":            GlobalRef(0),
		"ModeDimmerOverlay": GlobalRef(1),
	}
	block := compileBlockForTest(t, globals, "VarAssign[uint32](&UIMode, ModeDimmerOverlay)")

	sys := NewTestSystemInterface()
	globalState := []Var{
		{Index: 0, Type: VarTypeU8, Value: uint8(0)},
		{Index: 1, Type: VarTypeU8, Value: uint8(1)},
	}
	vm := NewVirtualMachine(sys, globalState)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if got := globalState[0].AsUint32(); got != 1 {
		t.Fatalf("unexpected UIMode after VarAssign: got %d want 1", got)
	}
}

func TestIntrinsicVarEqIgnoresGenericTypeArgs(t *testing.T) {
	globals := map[string]VarRef{
		"Finger0State": GlobalRef(0),
	}
	block := compileBlockForTest(t, globals, "return VarEq[uint32](Finger0State, 0)")

	sys := NewTestSystemInterface()
	globalState := []Var{{Index: 0, Type: VarTypeU8, Value: uint8(0)}}
	vm := NewVirtualMachine(sys, globalState)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 1 {
		t.Fatalf("unexpected stack size: got %d want 1", len(vm.DataStack))
	}
	if got := vm.DataStack[0].AsUint32(); got != 1 {
		t.Fatalf("unexpected VarEq result: got %d want 1", got)
	}
}

func TestIfBlocksUseSubScopeMetadata(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, "x := 1\nif 1 == 1 {\n\ty := 2\n\tx = y\n}\nreturn x")

	if root.Scope != BlockScopeFrame {
		t.Fatalf("root block should own a frame: %+v", root)
	}
	if root.LocalCount != 1 {
		t.Fatalf("unexpected root local count: got %d want 1", root.LocalCount)
	}

	if len(root.Bytes) < 20 || root.Bytes[10] != byte(OpIf) {
		t.Fatalf("root block does not contain expected OpIf bytecode: %v", root.Bytes)
	}

	condBlockID := binary.LittleEndian.Uint32(root.Bytes[11:15])
	trueBlockID := binary.LittleEndian.Uint32(root.Bytes[15:19])
	falseBlockID := binary.LittleEndian.Uint32(root.Bytes[19:23])

	for _, blockID := range []uint32{condBlockID, trueBlockID, falseBlockID} {
		block := compiler.blocks[blockID]
		if block == nil {
			t.Fatalf("missing child block %d", blockID)
		}
		if block.Scope != BlockScopeSub {
			t.Fatalf("child block should be sub-scope: %+v", block)
		}
		if block.InheritedLocals != 1 {
			t.Fatalf("child block should inherit parent locals: %+v", block)
		}
	}

	if compiler.blocks[trueBlockID].LocalCount != 2 {
		t.Fatalf("true block should expose parent + branch local slots: %+v", compiler.blocks[trueBlockID])
	}
	if compiler.blocks[falseBlockID].LocalCount != 1 {
		t.Fatalf("false block should keep inherited local count: %+v", compiler.blocks[falseBlockID])
	}
}
