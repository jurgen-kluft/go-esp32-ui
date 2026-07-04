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
	vm := NewVirtualMachine(sys)
	loadProgramIntoVM(vm, compiler)
	vm.ExecuteBlock(block.ID)

	if len(sys.DrawLog) != 1 {
		t.Fatalf("unexpected draw log length: got %d want 1", len(sys.DrawLog))
	}
	if got := sys.DrawLog[0]; got != "DrawBackground(Image: 7)" {
		t.Fatalf("unexpected draw log entry: %q", got)
	}
}
