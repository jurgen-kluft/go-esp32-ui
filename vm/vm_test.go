package vm

import (
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

func (s *TestSystemInterface) DrawText(fontID uint32, text string, x, y, color uint32) {
	s.DrawLog = append(s.DrawLog, fmt.Sprintf("DrawText(Font: %d, Text: %q, X: %d, Y: %d, Color: %d)", fontID, text, x, y, color))
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

func (s *TestSystemInterface) IsTimerDone(timerID uint32) bool {
	_, ok := s.Timers[timerID]
	return !ok
}

func (s *TestSystemInterface) SetLightOnOff(lightID, onOff uint32) {
	s.LightsOn[lightID] = onOff
}

func (s *TestSystemInterface) IsLightOn(lightID uint32) bool {
	return s.LightsOn[lightID] != 0
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
	vm := NewVirtualMachine(sys, nil)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 1 || vm.DataStack[0].Uint32Value() != 42 {
		t.Fatalf("unexpected stack after execution: %v", vm.DataStack)
	}
}

func TestCompileAndExecuteGlobalValue(t *testing.T) {
	globalID := GlobalRef(1)
	block := compileBlockForTest(t, map[string]VarRef{"g": globalID}, "return g")

	sys := NewTestSystemInterface()
	globalState := make([]Var, 2)
	globalState[globalID.Index] = Var{Index: uint16(globalID.Index), Type: VarTypeU32, Value: uint32(99)}
	vm := NewVirtualMachine(sys, globalState)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 1 || vm.DataStack[0].Uint32Value() != 99 {
		t.Fatalf("unexpected stack after execution: %v", vm.DataStack)
	}
}

func TestExecuteTaggedConstEncodings(t *testing.T) {
	systemInterface := NewCompilerSystemCalls()
	block := NewCompiler(nil, systemInterface).AllocateBlock()
	block.Bytes = []byte{
		byte(OpPushVar), 0x00, 0x00, 0x00, 0x01,
		byte(OpPushVar), 0x01, 0x00, 0x00, 0x01,
		byte(OpReturn), 2,
	}
	block.LocalCount = 0
	block.Consts = []Var{
		{Index: 0, Type: VarTypeU8, Flags: VarFlagConst, Value: uint8(42)},
		{Index: 1, Type: VarTypeU32, Flags: VarFlagConst, Value: uint32(256)},
	}

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 2 {
		t.Fatalf("unexpected stack length: got %d want 2, stack: %v", len(vm.DataStack), vm.DataStack)
	}
	if vm.DataStack[0].Uint32Value() != 42 {
		t.Fatalf("first stack value mismatch: got %d want 42", vm.DataStack[0].Uint32Value())
	}
	if vm.DataStack[1].Uint32Value() != 256 {
		t.Fatalf("second stack value mismatch: got %d want 256", vm.DataStack[1].Uint32Value())
	}
}

func TestExecuteIfReturnsToParentFrame(t *testing.T) {
	globalID := GlobalRef(2)
	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(map[string]VarRef{"g": globalID}, systemInterface)
	root := compiler.AllocateBlock()
	if err := compiler.CompileBlock(root, parseStatements(t, "if 1 == 1 {\ng = 11\n}\nreturn g")); err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	sys := NewTestSystemInterface()
	globalState := make([]Var, 3)
	globalState[globalID.Index] = Var{Index: uint16(globalID.Index), Type: VarTypeU32, Value: uint32(0)}
	vm := NewVirtualMachine(sys, globalState)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	if len(vm.DataStack) != 1 || vm.DataStack[0].Uint32Value() != 11 {
		t.Fatalf("unexpected stack after if execution: %v", vm.DataStack)
	}
	if got := globalState[globalID.Index]; !got.IsSet() || got.Uint32Value() != 11 {
		t.Fatalf("global state not updated by branch: got %+v", got)
	}
	if vm.CurrentFrame.BlockID != root.ID {
		t.Fatalf("current frame not restored to root: %+v", vm.CurrentFrame)
	}
	if len(vm.CallStack) != 0 {
		t.Fatalf("call stack not cleaned up: %v", vm.CallStack)
	}
}

func TestIfSubScopeFreesBranchLocalsAndKeepsParentLocals(t *testing.T) {
	globalID := GlobalRef(2)
	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(map[string]VarRef{"g": globalID}, systemInterface)
	root := compiler.AllocateBlock()
	if err := compiler.CompileBlock(root, parseStatements(t, "x := 3\nif 1 == 1 {\n\ty := 5\n\tx = y\n\tg = x\n}\nreturn x + g")); err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	sys := NewTestSystemInterface()
	globalState := make([]Var, 3)
	globalState[globalID.Index] = Var{Index: uint16(globalID.Index), Type: VarTypeU32, Value: uint32(0)}
	vm := NewVirtualMachine(sys, globalState)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	if len(vm.DataStack) != 1 || vm.DataStack[0].Uint32Value() != 10 {
		t.Fatalf("unexpected stack after scoped if execution: %v", vm.DataStack)
	}
	if got := globalState[globalID.Index]; !got.IsSet() || got.Uint32Value() != 5 {
		t.Fatalf("global state not updated by scoped branch: got %+v", got)
	}
	if vm.localTop != 0 {
		t.Fatalf("expected branch locals to be freed after execution, localTop=%d", vm.localTop)
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
		{name: "div", expr: "return 9 / 4", want: 2},
		{name: "div exact", expr: "return 9 / 3", want: 3},
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
			vm := NewVirtualMachine(sys, nil)
			vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
			vm.ExecuteBlock(block.ID)

			if len(vm.DataStack) != 1 || vm.DataStack[0].Uint32Value() != testCase.want {
				t.Fatalf("unexpected stack after execution: %v want %d", vm.DataStack, testCase.want)
			}
		})
	}
}

func TestCompileAndExecuteDivisionByZero(t *testing.T) {
	testCases := []struct {
		name      string
		expr      string
		wantType  VarType
		wantUint  uint32
		wantInt   int32
		wantFloat float32
	}{
		{name: "u32 saturates", expr: "return 9 / 0", wantType: VarTypeU32, wantUint: math.MaxUint32},
		{name: "s32 saturates positive", expr: "return -(-9) / 0", wantType: VarTypeS32, wantInt: int32(1<<31 - 1)},
		{name: "s32 saturates negative", expr: "return -9 / 0", wantType: VarTypeS32, wantInt: int32(-1 << 31)},
		{name: "f32 saturates positive", expr: "return 1.5 / 0", wantType: VarTypeF32, wantFloat: float32(math.Inf(1))},
		{name: "f32 saturates negative", expr: "return -1.5 / 0", wantType: VarTypeF32, wantFloat: float32(math.Inf(-1))},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			block := compileBlockForTest(t, nil, testCase.expr)

			sys := NewTestSystemInterface()
			vm := NewVirtualMachine(sys, nil)
			vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
			vm.ExecuteBlock(block.ID)

			if len(vm.DataStack) != 1 {
				t.Fatalf("unexpected stack after execution: %v", vm.DataStack)
			}
			got := vm.DataStack[0]
			if got.Type != testCase.wantType {
				t.Fatalf("unexpected var type: got %d want %d (%+v)", got.Type, testCase.wantType, got)
			}
			switch testCase.wantType {
			case VarTypeF32:
				if got.AsFloat32() != testCase.wantFloat {
					t.Fatalf("unexpected float result: got %f want %f", got.AsFloat32(), testCase.wantFloat)
				}
			case VarTypeS32:
				if got.AsInt32() != testCase.wantInt {
					t.Fatalf("unexpected signed result: got %d want %d", got.AsInt32(), testCase.wantInt)
				}
			default:
				if got.Uint32Value() != testCase.wantUint {
					t.Fatalf("unexpected unsigned result: got %d want %d", got.Uint32Value(), testCase.wantUint)
				}
			}
		})
	}
}

func TestCompileAndExecuteDrawSyscalls(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, strings.Join([]string{
		"DrawBackground(7)",
		"DrawSprite(8, 9, 10)",
		"DrawText(11, \"12\", 13, 14, 15)",
		"DrawVar(16, 17, 18, 19, 20)",
	}, "\n"))

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	want := []string{
		"DrawBackground(Image: 7)",
		"DrawSprite(Sprite: 8, X: 9, Y: 10)",
		"DrawText(Font: 11, Text: \"12\", X: 13, Y: 14, Color: 15)",
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
		"done := IsTimerDone(5)",
		"StopTimer(5)",
		"return done",
	}, "\n"))

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	if len(vm.DataStack) != 1 || vm.DataStack[0].Uint32Value() != 0 {
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
	vm := NewVirtualMachine(sys, nil)
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
	if len(vm.DataStack) != 1 || vm.DataStack[0].Uint32Value() != 123534 {
		t.Fatalf("unexpected light return stack: %v", vm.DataStack)
	}
}

func TestSyscallArgumentsUseNumericConversion(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, "DrawBackground(1.75)")

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	if len(sys.DrawLog) != 1 {
		t.Fatalf("unexpected draw log length: %d", len(sys.DrawLog))
	}
	if got := sys.DrawLog[0]; got != "DrawBackground(Image: 1)" {
		t.Fatalf("unexpected draw log entry: %q", got)
	}
}

func TestRuntimeConstantsBecomeConstVars(t *testing.T) {
	block := compileBlockForTest(t, nil, "return 42")

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 1 {
		t.Fatalf("unexpected stack after execution: %v", vm.DataStack)
	}
	result := vm.DataStack[0]
	if result.Type != VarTypeU8 || !result.HasFlag(VarFlagConst) {
		t.Fatalf("expected const u8 var result, got %+v", result)
	}
	if result.Uint32Value() != 42 {
		t.Fatalf("unexpected const value: got %d want 42", result.Uint32Value())
	}
}

func TestRuntimeTempsAndLocalsHaveVarIdentity(t *testing.T) {
	block := compileBlockForTest(t, nil, "x := 5\nreturn x + 2")

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 1 {
		t.Fatalf("unexpected stack after execution: %v", vm.DataStack)
	}
	result := vm.DataStack[0]
	if result.Index == 0 {
		t.Fatalf("expected temp result var to have a runtime identity: %+v", result)
	}
	if result.Type != VarTypeU32 || result.Uint32Value() != 7 {
		t.Fatalf("unexpected temp result var: %+v", result)
	}
	if result.HasFlag(VarFlagConst) {
		t.Fatalf("temp result should not be const: %+v", result)
	}
}
