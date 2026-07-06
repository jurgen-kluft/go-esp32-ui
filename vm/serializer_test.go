package vm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProgramImageRoundTripExecutesNumericProgram(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, "x := 42\nreturn x")

	image, err := compiler.WriteProgramImage(root.ID)
	if err != nil {
		t.Fatalf("WriteProgramImage failed: %v", err)
	}

	vm := NewVirtualMachine(NewTestSystemInterface(), nil)
	entryID, err := vm.LoadProgramImage(image)
	if err != nil {
		t.Fatalf("LoadProgramImage failed: %v", err)
	}
	if entryID != root.ID {
		t.Fatalf("unexpected entry block: got %d want %d", entryID, root.ID)
	}

	vm.ExecuteBlock(entryID)
	if vm.StackLen() != 1 {
		t.Fatalf("unexpected stack after execution: %s", vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 42)
}

func TestProgramImageRoundTripExecutesStringLiteralProgram(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, "DrawText(11, \"hello\", 13, 14, 15)")

	image, err := compiler.WriteProgramImage(root.ID)
	if err != nil {
		t.Fatalf("WriteProgramImage failed: %v", err)
	}

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	entryID, err := vm.LoadProgramImage(image)
	if err != nil {
		t.Fatalf("LoadProgramImage failed: %v", err)
	}

	vm.ExecuteBlock(entryID)
	if len(sys.DrawLog) != 1 {
		t.Fatalf("unexpected draw log length: got %d want 1", len(sys.DrawLog))
	}
	if got := sys.DrawLog[0]; got != "DrawText(Font: 11, Text: \"hello\", X: 13, Y: 14, Color: 15)" {
		t.Fatalf("unexpected draw log entry: %q", got)
	}

	block := vm.Blocks[root.ID]
	if len(block.Consts) != 5 {
		t.Fatalf("unexpected const count: got %d want 5", len(block.Consts))
	}
	if _, ok := block.Consts[1].Value.(programImageString); !ok {
		t.Fatalf("string const was not loaded from image payload: %T", block.Consts[1].Value)
	}
}

func TestProgramImageRoundTripExecutesComplexDeclaredGlobalsProgram(t *testing.T) {
	tempDir := t.TempDir()
	globalsPath := filepath.Join(tempDir, "globals.go")
	globalsSrc := "package ui\n\nvar (\n\tDateString = Var{Index: 4, Type: VarTypeStr, Flags: VarFlagConst | VarFlagPtr, Value: \"2026-07-05\"}\n\tFinger1X = Var{Index: 5, Type: VarTypeU16, Value: 0}\n\tTimerState = Var{Index: 6, Type: VarTypeU32, Value: 0}\n\tDisplayMode = Var{Index: 7, Type: VarTypeU8, Value: 0}\n)\n"
	if err := os.WriteFile(globalsPath, []byte(globalsSrc), 0o644); err != nil {
		t.Fatalf("write globals file failed: %v", err)
	}

	pagePath := filepath.Join(tempDir, "page.go")
	pageSrc := "package pages\n\nfunc Render() {\n\tvalue := uint32((VarToInt32(Finger1X)-5)/2)\n\tStartTimer(value, 25)\n\tif VarGe(value, 20) {\n\t\tDrawText(11, DateString, 13, value, 15)\n\t\tVarAssign(&DisplayMode, 3)\n\t\tVarAssign(&TimerState, value)\n\t\tDrawText(12, \"timer\", 14, 15, 16)\n\t}\n\tdone := IsTimerDone(value)\n\tStopTimer(value)\n\treturn done, DisplayMode, value\n}\n"
	if err := os.WriteFile(pagePath, []byte(pageSrc), 0o644); err != nil {
		t.Fatalf("write page file failed: %v", err)
	}

	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(nil, systemInterface)
	if err := compiler.LoadDeclaredGlobalsFromFile(globalsPath); err != nil {
		t.Fatalf("LoadDeclaredGlobalsFromFile failed: %v", err)
	}

	root := compiler.AllocateBlock()
	if err := compiler.CompileFunctionFile(root, pagePath, "Render"); err != nil {
		t.Fatalf("CompileFunctionFile failed: %v", err)
	}

	image, err := compiler.WriteProgramImage(root.ID)
	if err != nil {
		t.Fatalf("WriteProgramImage failed: %v", err)
	}

	sys := NewTestSystemInterface()
	globalState := make([]Var, 8)
	globalState[4] = Var{Index: 4, Type: VarTypeStr, Flags: VarFlagConst | VarFlagPtr, Value: "2026-07-05"}
	globalState[5] = Var{Index: 5, Type: VarTypeU16, Value: uint16(45)}
	globalState[6] = Var{Index: 6, Type: VarTypeU32, Value: uint32(0)}
	globalState[7] = Var{Index: 7, Type: VarTypeU8, Value: uint8(1)}
	vm := NewVirtualMachine(sys, globalState)
	entryID, err := vm.LoadProgramImage(image)
	if err != nil {
		t.Fatalf("LoadProgramImage failed: %v", err)
	}
	if entryID != root.ID {
		t.Fatalf("unexpected entry block: got %d want %d", entryID, root.ID)
	}

	vm.ExecuteBlock(entryID)
	if vm.StackLen() != 3 {
		t.Fatalf("unexpected stack after execution: %s", vm.DumpRefStack())
	}
	requireStackUint32(t, vm, 0, 0)
	requireStackUint32(t, vm, 1, 3)
	requireStackUint32(t, vm, 2, 20)

	if got := globalState[6].Uint32Value(); got != 20 {
		t.Fatalf("unexpected TimerState: got %d want 20", got)
	}
	if got := globalState[7].Uint32Value(); got != 3 {
		t.Fatalf("unexpected DisplayMode: got %d want 3", got)
	}
	if _, ok := sys.Timers[20]; ok {
		t.Fatalf("timer 20 should have been stopped: %v", sys.Timers)
	}
	if len(sys.DrawLog) != 2 {
		t.Fatalf("unexpected draw log length: got %d want 2", len(sys.DrawLog))
	}
	if got := sys.DrawLog[0]; got != "DrawText(Font: 11, Text: \"2026-07-05\", X: 13, Y: 20, Color: 15)" {
		t.Fatalf("unexpected first draw log entry: %q", got)
	}
	if got := sys.DrawLog[1]; got != "DrawText(Font: 12, Text: \"timer\", X: 14, Y: 15, Color: 16)" {
		t.Fatalf("unexpected second draw log entry: %q", got)
	}

	block := vm.Blocks[root.ID]
	hasImageString := false
	for _, variable := range block.Consts {
		if _, ok := variable.Value.(programImageString); ok {
			hasImageString = true
			break
		}
	}
	if !hasImageString {
		t.Fatal("expected loaded block constants to include a serialized string payload")
	}
}

func TestLoadProgramImageRejectsMissingIfTarget(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, "if 1 == 1 { return 1 }\nreturn 0")

	image, err := compiler.WriteProgramImage(root.ID)
	if err != nil {
		t.Fatalf("WriteProgramImage failed: %v", err)
	}

	mutated := append([]byte(nil), image...)
	header, err := readProgramImageHeader(mutated)
	if err != nil {
		t.Fatalf("readProgramImageHeader failed: %v", err)
	}
	rootRecord, err := readProgramImageBlockRecord(mutated, header.BlockTableOffset)
	if err != nil {
		t.Fatalf("readProgramImageBlockRecord failed: %v", err)
	}
	if len(mutated) < int(rootRecord.BytecodeOffset+9) {
		t.Fatalf("unexpected bytecode layout for root block")
	}
	putU32(mutated[rootRecord.BytecodeOffset+1:rootRecord.BytecodeOffset+5], 9999)

	vm := NewVirtualMachine(NewTestSystemInterface(), nil)
	if _, err := vm.LoadProgramImage(mutated); err == nil {
		t.Fatalf("LoadProgramImage succeeded for image with missing if target")
	}
}