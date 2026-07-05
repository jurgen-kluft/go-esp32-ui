package vm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoFileAndFindFunction(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "page.go")
	src := "package pages\n\nfunc Alpha() {\n\tx := 1\n\t_ = x\n}\n\nfunc Beta() {\n\tDrawBackground(7)\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	_, file, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile failed: %v", err)
	}

	fn, err := FindFunction(file, "Beta")
	if err != nil {
		t.Fatalf("FindFunction failed: %v", err)
	}

	stmts, err := FunctionStatements(fn)
	if err != nil {
		t.Fatalf("FunctionStatements failed: %v", err)
	}

	if fn.Name == nil || fn.Name.Name != "Beta" {
		t.Fatalf("unexpected function name: %+v", fn.Name)
	}
	if len(stmts) != 1 {
		t.Fatalf("unexpected statement count: got %d want 1", len(stmts))
	}
}

func TestCompileFunctionFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "page.go")
	src := "package pages\n\nfunc Render() {\n\tDrawBackground(7)\n\treturn\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(nil, systemInterface)
	block := compiler.AllocateBlock()
	if err := compiler.CompileFunctionFile(block, path, "Render"); err != nil {
		t.Fatalf("CompileFunctionFile failed: %v", err)
	}

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(block.ID)

	if len(sys.DrawLog) != 1 {
		t.Fatalf("unexpected draw log length: got %d want 1", len(sys.DrawLog))
	}
	if got := sys.DrawLog[0]; got != "DrawBackground(Image: 7)" {
		t.Fatalf("unexpected draw log entry: %q", got)
	}
}

func TestParseDeclaredGlobals(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "globals.go")
	src := "package ui\n\nvar (\n\tDateString = Var{Index: 4, Type: VarTypeStr, Flags: VarFlagConst | VarFlagPtr, Value: \"2026-07-05\"}\n\tFinger0State = Var{Index: 6, Type: VarTypeU8, Value: 0}\n)\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	_, file, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile failed: %v", err)
	}

	globals, err := ParseDeclaredGlobals(file)
	if err != nil {
		t.Fatalf("ParseDeclaredGlobals failed: %v", err)
	}

	dateString, ok := globals["DateString"]
	if !ok {
		t.Fatal("DateString global not discovered")
	}
	if dateString.Type != VarTypeStr || !dateString.HasFlag(VarFlagConst) || !dateString.HasFlag(VarFlagPtr) {
		t.Fatalf("unexpected DateString metadata: %+v", dateString)
	}
	if value, ok := dateString.Value.(string); !ok || value != "2026-07-05" {
		t.Fatalf("unexpected DateString value: %#v", dateString.Value)
	}

	finger0State, ok := globals["Finger0State"]
	if !ok {
		t.Fatal("Finger0State global not discovered")
	}
	if finger0State.Index != 6 || finger0State.Type != VarTypeU8 {
		t.Fatalf("unexpected Finger0State metadata: %+v", finger0State)
	}
}

func TestCompilerLoadsDeclaredGlobalsFromFile(t *testing.T) {
	tempDir := t.TempDir()
	globalsPath := filepath.Join(tempDir, "globals.go")
	globalsSrc := "package ui\n\nvar LightState = Var{Index: 7, Type: VarTypeU8, Value: 0}\n"
	if err := os.WriteFile(globalsPath, []byte(globalsSrc), 0o644); err != nil {
		t.Fatalf("write globals file failed: %v", err)
	}

	pagePath := filepath.Join(tempDir, "page.go")
	pageSrc := "package pages\n\nfunc Render() {\n\treturn LightState\n}\n"
	if err := os.WriteFile(pagePath, []byte(pageSrc), 0o644); err != nil {
		t.Fatalf("write page file failed: %v", err)
	}

	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(nil, systemInterface)
	if err := compiler.LoadDeclaredGlobalsFromFile(globalsPath); err != nil {
		t.Fatalf("LoadDeclaredGlobalsFromFile failed: %v", err)
	}

	block := compiler.AllocateBlock()
	if err := compiler.CompileFunctionFile(block, pagePath, "Render"); err != nil {
		t.Fatalf("CompileFunctionFile failed: %v", err)
	}

	sys := NewTestSystemInterface()
	globalState := make([]Var, 8)
	globalState[7] = Var{Index: 7, Type: VarTypeU8, Value: uint8(99)}
	vm := NewVirtualMachine(sys, globalState)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(block.ID)

	if len(vm.DataStack) != 1 || vm.DataStack[0].Uint32Value() != 99 {
		t.Fatalf("unexpected stack after execution: %v", vm.DataStack)
	}
}

func TestCompileAndExecuteDrawTextWithStringLiteral(t *testing.T) {
	compiler, root := compileProgramForTest(t, nil, "DrawText(11, \"hello\", 13, 14, 15)")

	sys := NewTestSystemInterface()
	vm := NewVirtualMachine(sys, nil)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(root.ID)

	if len(sys.DrawLog) != 1 {
		t.Fatalf("unexpected draw log length: got %d want 1", len(sys.DrawLog))
	}
	if got := sys.DrawLog[0]; got != "DrawText(Font: 11, Text: \"hello\", X: 13, Y: 14, Color: 15)" {
		t.Fatalf("unexpected draw log entry: %q", got)
	}
	constant, ok := compiler.stringConstants["hello"]
	if !ok || constant.Type != VarTypeStr || !constant.HasFlag(VarFlagConst) || !constant.HasFlag(VarFlagPtr) {
		t.Fatalf("string constant not interned correctly: %+v", constant)
	}
}

func TestCompileAndExecuteDrawTextWithDeclaredStringGlobal(t *testing.T) {
	tempDir := t.TempDir()
	globalsPath := filepath.Join(tempDir, "globals.go")
	globalsSrc := "package ui\n\nvar DateString = Var{Index: 4, Type: VarTypeStr, Flags: VarFlagConst | VarFlagPtr, Value: \"2026-07-05\"}\n"
	if err := os.WriteFile(globalsPath, []byte(globalsSrc), 0o644); err != nil {
		t.Fatalf("write globals file failed: %v", err)
	}

	pagePath := filepath.Join(tempDir, "page.go")
	pageSrc := "package pages\n\nfunc Render() {\n\tDrawText(11, DateString, 13, 14, 15)\n}\n"
	if err := os.WriteFile(pagePath, []byte(pageSrc), 0o644); err != nil {
		t.Fatalf("write page file failed: %v", err)
	}

	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(nil, systemInterface)
	if err := compiler.LoadDeclaredGlobalsFromFile(globalsPath); err != nil {
		t.Fatalf("LoadDeclaredGlobalsFromFile failed: %v", err)
	}

	block := compiler.AllocateBlock()
	if err := compiler.CompileFunctionFile(block, pagePath, "Render"); err != nil {
		t.Fatalf("CompileFunctionFile failed: %v", err)
	}

	sys := NewTestSystemInterface()
	globalState := make([]Var, 5)
	globalState[4] = Var{Index: 4, Type: VarTypeStr, Flags: VarFlagConst | VarFlagPtr, Value: "2026-07-05"}
	vm := NewVirtualMachine(sys, globalState)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(block.ID)

	if len(sys.DrawLog) != 1 {
		t.Fatalf("unexpected draw log length: got %d want 1", len(sys.DrawLog))
	}
	if got := sys.DrawLog[0]; got != "DrawText(Font: 11, Text: \"2026-07-05\", X: 13, Y: 14, Color: 15)" {
		t.Fatalf("unexpected draw log entry: %q", got)
	}
}

