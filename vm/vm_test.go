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

func (s *TestSystemInterface) RegisterZone(zoneID, x, y, width, height uint32, gesture uint8) bool {
	return true
}

func (s *TestSystemInterface) TurnRelayOnOff(relay int8, status int8) {
}

func (s *TestSystemInterface) SetDisplayBrightness(brightness uint8) {
}

// -----------------------------------------------------------------------------------------------------
// -----------------------------------------------------------------------------------------------------

func TestCompileAndExecuteLocalValue(t *testing.T) {
	block := compileBlockForTest(t, nil, "x := 42\nreturn x")

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if vm.StackLen() != 1 {
		t.Fatalf("unexpected stack after execution: %s", vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 42)
}

func TestReturnValueSurvivesLocalArenaReuse(t *testing.T) {
	block := compileBlockForTest(t, nil, "x := 42\nreturn x")

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if vm.StackLen() != 1 {
		t.Fatalf("unexpected stack after execution: %s", vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 42)

	vm.allocLocalVar(0).Assign(uint32(7))

	if got := requireStackValue(t, vm, 0).Uint32Value(); got != 42 {
		t.Fatalf("returned value changed after local arena reuse: got %d want 42", got)
	}
}

func TestResolveStackRefSupportsAllRuntimeStorages(t *testing.T) {
	sys := NewTestSystemInterface()
	globalState := []Var{{Index: 0, Type: VarTypeU32, Value: uint32(11)}}
	vm := NewVirtualMachine(sys, globalState)
	block := VMBlock{Consts: []Var{{Index: 0, Type: VarTypeU32, Flags: VarFlagConst, Value: uint32(22)}}}

	vm.CurrentFrame = CallFrame{LocalBase: 0}
	local := vm.allocLocalVar(0)
	local.Assign(uint32(33))
	vm.pushVarWithRef(local, StackRefFromVarRef(LocalRef(0), vm.allocScopeID()))
	liveLocalRef := vm.RefStack[0]

	temp := vm.allocTempVar(VarTypeU32, VarFlagNone, uint32(44))
	vm.pushTempVar(temp)
	liveTempRef := vm.RefStack[len(vm.RefStack)-1]

	testCases := []struct {
		name string
		ref  StackRef
		want uint32
	}{
		{name: "global", ref: StackRefFromVarRef(GlobalRef(0), 0), want: 11},
		{name: "const", ref: StackRefFromVarRef(ConstRef(0), 0), want: 22},
		{name: "local", ref: liveLocalRef, want: 33},
		{name: "temp", ref: liveTempRef, want: 44},
	}

	for _, testCase := range testCases {
		resolved, ok := vm.resolveStackRef(block, testCase.ref)
		if !ok {
			t.Fatalf("%s ref did not resolve", testCase.name)
		}
		if got := resolved.Uint32Value(); got != testCase.want {
			t.Fatalf("%s ref mismatch: got %d want %d", testCase.name, got, testCase.want)
		}
	}
}

func TestEnterBlockPushesLocalRefs(t *testing.T) {
	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	vm.Blocks[7] = VMBlock{ID: 7, LocalCount: 2}

	frame := vm.enterBlock(7, nil)
	vm.CurrentFrame = frame

	if vm.StackLen() != 2 || len(vm.RefStack) != 2 {
		t.Fatalf("unexpected stack lengths: stack=%d refs=%d", vm.StackLen(), len(vm.RefStack))
	}
	for idx, ref := range vm.RefStack {
		if ref.Storage != LocalRefType || ref.Index != uint32(idx) {
			t.Fatalf("unexpected local stack ref at %d: %+v", idx, ref)
		}
		if ref.ScopeID == 0 {
			t.Fatalf("expected local scope id at %d, got %+v", idx, ref)
		}
	}
}

func TestResolveStackRefRejectsStaleTempGeneration(t *testing.T) {
	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	block := VMBlock{}

	firstTemp := vm.allocTempVar(VarTypeU32, VarFlagNone, uint32(44))
	vm.pushTempVar(firstTemp)
	staleRef := vm.RefStack[len(vm.RefStack)-1]

	vm.truncateStacks(0)
	vm.tempTop = 0

	secondTemp := vm.allocTempVar(VarTypeU32, VarFlagNone, uint32(55))
	vm.pushTempVar(secondTemp)
	freshRef := vm.RefStack[len(vm.RefStack)-1]

	if _, ok := vm.resolveStackRef(block, staleRef); ok {
		t.Fatalf("expected stale temp ref to fail after slot reuse: %+v", staleRef)
	}

	resolved, ok := vm.resolveStackRef(block, freshRef)
	if !ok {
		t.Fatalf("expected fresh temp ref to resolve: %+v", freshRef)
	}
	if got := resolved.Uint32Value(); got != 55 {
		t.Fatalf("unexpected fresh temp value: got %d want 55", got)
	}
}

func TestExecuteBlockPushesReturnedTempRef(t *testing.T) {
	block := compileBlockForTest(t, nil, "x := 42\nreturn x")

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	vm.Blocks[block.ID] = VMBlock{ID: block.ID, LocalCount: block.LocalCount, Bytes: block.Bytes, Consts: block.Consts}
	vm.ExecuteBlock(block.ID)

	if vm.StackLen() != 1 || len(vm.RefStack) != 1 {
		t.Fatalf("unexpected stack lengths: stack=%d refs=%d", vm.StackLen(), len(vm.RefStack))
	}
	ref := vm.RefStack[0]
	if ref.Storage != TempRefType {
		t.Fatalf("expected returned value to be pushed as temp ref, got %+v", ref)
	}
	resolved, ok := vm.resolveStackRef(vm.Blocks[block.ID], ref)
	if !ok {
		t.Fatalf("returned temp ref did not resolve: %+v", ref)
	}
	if got := resolved.Uint32Value(); got != 42 {
		t.Fatalf("unexpected resolved returned value: got %d want 42", got)
	}
}

func TestDumpRefStackShowsStorageProvenance(t *testing.T) {
	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, []Var{{Index: 0, Type: VarTypeU32, Value: uint32(11)}})
	block := VMBlock{Consts: []Var{{Index: 0, Type: VarTypeU32, Flags: VarFlagConst, Value: uint32(22)}}}
	vm.CurrentFrame = CallFrame{BlockID: 9, LocalBase: 0}

	local := vm.allocLocalVar(0)
	local.Assign(uint32(33))
	vm.pushVarWithRef(&vm.Globals[0], StackRefFromVarRef(GlobalRef(0), vm.CurrentFrame.BlockID))
	vm.pushVarWithRef(&block.Consts[0], StackRefFromVarRef(ConstRef(0), vm.CurrentFrame.BlockID))
	vm.pushVarWithRef(local, StackRefFromVarRef(LocalRef(0), vm.CurrentFrame.BlockID))
	temp := vm.allocTempVar(VarTypeU32, VarFlagNone, uint32(44))
	vm.pushVarWithRef(temp, StackRefFromVarRef(TempRef(uint32(temp.Index-1)), vm.CurrentFrame.BlockID))

	if got, want := vm.DumpRefStack(), "[Global(0), Const(0), Local(0)@9, Temp(0)@9]"; got != want {
		t.Fatalf("unexpected ref stack dump: got %q want %q", got, want)
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

	if vm.StackLen() != 1 {
		t.Fatalf("unexpected stack after execution: %s", vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 99)
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

	if vm.StackLen() != 2 {
		t.Fatalf("unexpected stack length: got %d want 2, stack: %s", vm.StackLen(), vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 42)
	requireStackUint32(t, vm, 1, 256)
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

	if vm.StackLen() != 1 {
		t.Fatalf("unexpected stack after if execution: %s", vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 11)
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

	if vm.StackLen() != 1 {
		t.Fatalf("unexpected stack after scoped if execution: %s", vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 10)
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

			if vm.StackLen() != 1 {
				t.Fatalf("unexpected stack after execution: %s want %d", vm.DumpRefStack(), testCase.want)
			}
			requireStackUint32(t, vm, 0, testCase.want)
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

			if vm.StackLen() != 1 {
				t.Fatalf("unexpected stack after execution: %s", vm.DumpRefStack())
			}
			got := requireStackValue(t, vm, 0)
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

	if vm.StackLen() != 1 {
		t.Fatalf("unexpected timer return stack: %s", vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 0)
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
	if vm.StackLen() != 1 {
		t.Fatalf("unexpected light return stack: %s", vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 123534)
}

func TestCompileAndExecuteComplexLightingProgram(t *testing.T) {
	globals := map[string]VarRef{
		"Finger1X":   GlobalRef(0),
		"UIMode":     GlobalRef(1),
		"LightLevel": GlobalRef(2),
		"StatusVar":  GlobalRef(3),
	}

	compiler, root := compileProgramForTest(t, globals, strings.Join([]string{
		"level := uint32((VarToInt32(Finger1X)-10)*2)",
		"VarAssign(&LightLevel, level)",
		"if VarGt(level, 40) {",
		"\tSetLightOnOff(1, 1)",
		"\tSetLightBrightness(1, level)",
		"\tSetLightColor(1, 123456)",
		"\tDrawText(11, \"bright\", 13, level, 15)",
		"\tVarAssign(&UIMode, 2)",
		"\tstatus := GetLightBrightness(1)",
		"\tVarAssign(&StatusVar, status)",
		"}",
		"return level, UIMode, LightLevel",
	}, "\n"))

	sys := NewTestSystemInterface()
	globalState := []Var{
		{Index: 0, Type: VarTypeU16, Value: uint16(35)},
		{Index: 1, Type: VarTypeU8, Value: uint8(0)},
		{Index: 2, Type: VarTypeU32, Value: uint32(0)},
		{Index: 3, Type: VarTypeU32, Value: uint32(0)},
	}
	vm := NewVirtualMachine(sys, globalState)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	if vm.StackLen() != 3 {
		t.Fatalf("unexpected stack after execution: %s", vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 50)
	requireStackUint32(t, vm, 1, 2)
	requireStackUint32(t, vm, 2, 50)

	if got := globalState[1].Uint32Value(); got != 2 {
		t.Fatalf("unexpected UIMode: got %d want 2", got)
	}
	if got := globalState[2].Uint32Value(); got != 50 {
		t.Fatalf("unexpected LightLevel: got %d want 50", got)
	}
	if got := globalState[3].Uint32Value(); got != 50 {
		t.Fatalf("unexpected StatusVar: got %d want 50", got)
	}
	if got := sys.LightsOn[1]; got != 1 {
		t.Fatalf("unexpected light state: got %d want 1", got)
	}
	if got := sys.LightBrightness[1]; got != 50 {
		t.Fatalf("unexpected brightness: got %d want 50", got)
	}
	if got := sys.LightColor[1]; got != 123456 {
		t.Fatalf("unexpected color: got %d want 123456", got)
	}
	if len(sys.DrawLog) != 1 {
		t.Fatalf("unexpected draw log length: got %d want 1", len(sys.DrawLog))
	}
	if got := sys.DrawLog[0]; got != "DrawText(Font: 11, Text: \"bright\", X: 13, Y: 50, Color: 15)" {
		t.Fatalf("unexpected draw log entry: %q", got)
	}
	if vm.localTop != 0 {
		t.Fatalf("expected locals to be released after execution, localTop=%d", vm.localTop)
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

	if vm.StackLen() != 1 {
		t.Fatalf("unexpected stack after execution: %s", vm.DumpRefStack())
	}
	result := requireStackValue(t, vm, 0)
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

	if vm.StackLen() != 1 {
		t.Fatalf("unexpected stack after execution: %s", vm.DumpRefStack())
	}
	result := requireStackValue(t, vm, 0)
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
