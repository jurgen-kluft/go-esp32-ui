package vm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

type TestSystemInterface struct {
	DrawLog         []string
	Timers          map[uint32]uint32
	LightsOn        map[uint32]uint32
	LightBrightness map[uint32]uint32
	LightColor      map[uint32]uint32
}

func NewTestSystemInterface() *TestSystemInterface {
	return &TestSystemInterface{
		DrawLog:         make([]string, 0, 16),
		Timers:          make(map[uint32]uint32),
		LightsOn:        make(map[uint32]uint32),
		LightBrightness: make(map[uint32]uint32),
		LightColor:      make(map[uint32]uint32),
	}
}

func (s *TestSystemInterface) DrawBackground(imageID uint32) {
	s.DrawLog = append(s.DrawLog, fmt.Sprintf("DrawBackground(Image: %d)", imageID))
}
func (s *TestSystemInterface) DrawSprite(spriteID, x, y uint32) {
	s.DrawLog = append(s.DrawLog, fmt.Sprintf("DrawSprite(Sprite: %d, X: %d, Y: %d)", spriteID, x, y))
}
func (s *TestSystemInterface) DrawText(fontID, textID, x, y, color uint32) {
	s.DrawLog = append(s.DrawLog, fmt.Sprintf("DrawText(Font: %d, Text: %d, X: %d, Y: %d, Color: %d)", fontID, textID, x, y, color))
}
func (s *TestSystemInterface) DrawVar(fontID, varID, x, y, color uint32) {
	s.DrawLog = append(s.DrawLog, fmt.Sprintf("DrawVar(Font: %d, Var: %d, X: %d, Y: %d, Color: %d)", fontID, varID, x, y, color))
}
func (s *TestSystemInterface) StartTimer(timerID, duration uint32) {
	s.Timers[timerID] = duration
}
func (s *TestSystemInterface) StopTimer(timerID uint32) {
	delete(s.Timers, timerID)
}
func (s *TestSystemInterface) GetTimer(timerID uint32) uint32 {
	return s.Timers[timerID]
}
func (s *TestSystemInterface) SetLightOnOff(lightID, onOff uint32) {
	s.LightsOn[lightID] = onOff
}
func (s *TestSystemInterface) IsLightOn(lightID uint32) uint32 {
	return s.LightsOn[lightID]
}
func (s *TestSystemInterface) SetLightBrightness(lightID, brightness uint32) {
	s.LightBrightness[lightID] = brightness
}
func (s *TestSystemInterface) GetLightBrightness(lightID uint32) uint32 {
	return s.LightBrightness[lightID]
}
func (s *TestSystemInterface) SetLightColor(lightID, color uint32) {
	s.LightColor[lightID] = color
}
func (s *TestSystemInterface) GetLightColor(lightID uint32) uint32 {
	return s.LightColor[lightID]
}

// -----------------------------------------------------------------------------------------------------
// -----------------------------------------------------------------------------------------------------

func TestCompileAndExecuteLocalValue(t *testing.T) {
	block := compileBlockForTest(t, nil, "x := 42\nreturn x")

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes}
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 1 || vm.DataStack[0].Bits != 42 {
		t.Fatalf("unexpected stack after execution: %v", vm.DataStack)
	}
}

func TestCompileAndExecuteGlobalValue(t *testing.T) {
	globalID := ID{Type: 3, Idx: 1}
	block := compileBlockForTest(t, map[string]ID{"g": globalID}, "return g")

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys)
	vm.GlobalState[globalID.Pack()] = 99
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes}
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 1 || vm.DataStack[0].Bits != 99 {
		t.Fatalf("unexpected stack after execution: %v", vm.DataStack)
	}
}

func TestExecuteTaggedConstEncodings(t *testing.T) {
	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(nil, systemInterface)
	block := compiler.AllocateBlock()

	var buf bytes.Buffer
	buf.WriteByte(byte(OpPushConst))
	buf.WriteByte(byte(ConstTypeU8))
	buf.WriteByte(42)
	buf.WriteByte(byte(OpPushConst))
	buf.WriteByte(byte(ConstTypeU32))
	binary.Write(&buf, binary.LittleEndian, uint32(256))
	buf.WriteByte(byte(OpReturn))
	buf.WriteByte(2)

	block.Bytes = buf.Bytes()
	block.LocalCount = 0

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes}
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 2 {
		t.Fatalf("unexpected stack length: got %d want 2, stack: %v", len(vm.DataStack), vm.DataStack)
	}
	if vm.DataStack[0].Bits != 42 {
		t.Fatalf("first stack value mismatch: got %d want 42", vm.DataStack[0].Bits)
	}
	if vm.DataStack[1].Bits != 256 {
		t.Fatalf("second stack value mismatch: got %d want 256", vm.DataStack[1].Bits)
	}
}

func TestExecuteIfReturnsToParentFrame(t *testing.T) {
	globalID := ID{Type: 3, Idx: 2}
	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(map[string]ID{"g": globalID}, systemInterface)
	root := compiler.AllocateBlock()
	if err := compiler.CompileBlock(root, parseStatements(t, "if 1 == 1 {\ng = 11\n}\nreturn g")); err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys)
	vm.GlobalState[globalID.Pack()] = 0
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	if len(vm.DataStack) != 1 || vm.DataStack[0].Bits != 11 {
		t.Fatalf("unexpected stack after if execution: %v", vm.DataStack)
	}
	if got := vm.GlobalState[globalID.Pack()]; got != 11 {
		t.Fatalf("global state not updated by branch: got %d", got)
	}
	if vm.CurrentFrame.BlockID != root.ID {
		t.Fatalf("current frame not restored to root: %+v", vm.CurrentFrame)
	}
	if len(vm.CallStack) != 0 {
		t.Fatalf("call stack not cleaned up: %v", vm.CallStack)
	}
}

func TestCompileAndExecuteBinaryOperators(t *testing.T) {
	testCases := []struct {
		name string
		expr string
		want uint32
	}{
		{name: "add", expr: "return 9 + 4", want: 13},
		{name: "sub", expr: "return 9 - 4", want: 5},
		{name: "mul", expr: "return 9 * 4", want: 36},
		{name: "eq true", expr: "return 9 == 9", want: 1},
		{name: "eq false", expr: "return 9 == 4", want: 0},
		{name: "gt", expr: "return 9 > 4", want: 1},
		{name: "ge true", expr: "return 9 >= 9", want: 1},
		{name: "ge false", expr: "return 4 >= 9", want: 0},
		{name: "lt", expr: "return 4 < 9", want: 1},
		{name: "le true", expr: "return 4 <= 4", want: 1},
		{name: "le false", expr: "return 9 <= 4", want: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			block := compileBlockForTest(t, nil, testCase.expr)

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

func TestCompileAndExecuteDrawSyscalls(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, strings.Join([]string{
		"DrawBackground(7)",
		"DrawSprite(8, 9, 10)",
		"DrawText(11, 12, 13, 14, 15)",
		"DrawVar(16, 17, 18, 19, 20)",
	}, "\n"))

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	want := []string{
		"DrawBackground(Image: 7)",
		"DrawSprite(Sprite: 8, X: 9, Y: 10)",
		"DrawText(Font: 11, Text: 12, X: 13, Y: 14, Color: 15)",
		"DrawVar(Font: 16, Var: 17, X: 18, Y: 19, Color: 20)",
	}
	if len(sys.DrawLog) != len(want) {
		t.Fatalf("unexpected draw log length: got %d want %d", len(sys.DrawLog), len(want))
	}
	for i := range want {
		if sys.DrawLog[i] != want[i] {
			t.Fatalf("draw log mismatch at %d: got %q want %q", i, sys.DrawLog[i], want[i])
		}
	}
}

func TestCompileAndExecuteTimerSyscalls(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, strings.Join([]string{
		"StartTimer(5, 250)",
		"remaining := GetTimer(5)",
		"StopTimer(5)",
		"return remaining",
	}, "\n"))

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	if len(vm.DataStack) != 1 || vm.DataStack[0].Bits != 250 {
		t.Fatalf("unexpected timer return stack: %v", vm.DataStack)
	}
	if _, ok := sys.Timers[5]; ok {
		t.Fatalf("timer 5 should have been stopped: %v", sys.Timers)
	}
}

func TestCompileAndExecuteLightStateSyscalls(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, strings.Join([]string{
		"SetLightOnOff(1, 1)",
		"SetLightBrightness(1, 77)",
		"SetLightColor(1, 123456)",
		"on := IsLightOn(1)",
		"brightness := GetLightBrightness(1)",
		"color := GetLightColor(1)",
		"return on + brightness + color",
	}, "\n"))

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	if got := sys.LightsOn[1]; got != 1 {
		t.Fatalf("unexpected light state: %d", got)
	}
	if got := sys.LightBrightness[1]; got != 77 {
		t.Fatalf("unexpected brightness: %d", got)
	}
	if got := sys.LightColor[1]; got != 123456 {
		t.Fatalf("unexpected color: %d", got)
	}
	if len(vm.DataStack) != 1 || vm.DataStack[0].Bits != 123534 {
		t.Fatalf("unexpected light return stack: %v", vm.DataStack)
	}
}

func TestValueAccessorsRespectKind(t *testing.T) {
	testCases := []struct {
		name      string
		value     Value
		wantInt32 int32
		wantUint  uint32
		wantFloat float32
	}{
		{
			name:      "u32",
			value:     Value{Kind: ValueKindU32, Bits: 42},
			wantInt32: 42,
			wantUint:  42,
			wantFloat: 42,
		},
		{
			name:      "s32",
			value:     Value{Kind: ValueKindS32, Bits: ^uint32(6)},
			wantInt32: -7,
			wantUint:  ^uint32(6),
			wantFloat: -7,
		},
		{
			name:      "f32",
			value:     Value{Kind: ValueKindF32, Bits: math.Float32bits(1.75)},
			wantInt32: 1,
			wantUint:  1,
			wantFloat: 1.75,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.value.AsInt32(); got != testCase.wantInt32 {
				t.Fatalf("AsInt32 mismatch: got %d want %d", got, testCase.wantInt32)
			}
			if got := testCase.value.AsUint32(); got != testCase.wantUint {
				t.Fatalf("AsUint32 mismatch: got %d want %d", got, testCase.wantUint)
			}
			if got := testCase.value.AsFloat32(); got != testCase.wantFloat {
				t.Fatalf("AsFloat32 mismatch: got %f want %f", got, testCase.wantFloat)
			}
		})
	}
}

func TestValueSettersRespectKind(t *testing.T) {
	var value Value

	value.SetInt8(-5)
	if value.Kind != ValueKindS32 || value.AsInt32() != -5 {
		t.Fatalf("SetInt8 mismatch: %+v", value)
	}

	value.SetInt16(-257)
	if value.Kind != ValueKindS32 || value.AsInt32() != -257 {
		t.Fatalf("SetInt16 mismatch: %+v", value)
	}

	value.SetInt32(-1024)
	if value.Kind != ValueKindS32 || value.AsInt32() != -1024 {
		t.Fatalf("SetInt32 mismatch: %+v", value)
	}

	value.SetUint8(7)
	if value.Kind != ValueKindU32 || value.AsUint32() != 7 {
		t.Fatalf("SetUint8 mismatch: %+v", value)
	}

	value.SetUint16(513)
	if value.Kind != ValueKindU32 || value.AsUint32() != 513 {
		t.Fatalf("SetUint16 mismatch: %+v", value)
	}

	value.SetUint32(65537)
	if value.Kind != ValueKindU32 || value.AsUint32() != 65537 {
		t.Fatalf("SetUint32 mismatch: %+v", value)
	}

	value.SetFloat32(2.5)
	if value.Kind != ValueKindF32 || value.AsFloat32() != 2.5 {
		t.Fatalf("SetFloat32 mismatch: %+v", value)
	}
}

func TestSyscallArgumentsUseNumericConversion(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, "DrawBackground(1.75)")

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	if len(sys.DrawLog) != 1 {
		t.Fatalf("unexpected draw log length: %d", len(sys.DrawLog))
	}
	if got := sys.DrawLog[0]; got != "DrawBackground(Image: 1)" {
		t.Fatalf("unexpected draw log entry: %q", got)
	}
}
