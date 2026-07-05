package vm

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func requireStackValue(t *testing.T, vm *VM, index uint32) *Var {
	t.Helper()

	value, ok := vm.StackValueAt(index)
	if !ok {
		t.Fatalf("stack value %d did not resolve; refs=%s", index, vm.DumpRefStack())
	}
	return value
}

func requireStackUint32(t *testing.T, vm *VM, index uint32, want uint32) {
	t.Helper()

	if got := requireStackValue(t, vm, index).Uint32Value(); got != want {
		t.Fatalf("unexpected stack value at %d: got %d want %d", index, got, want)
	}
}

func parseStatements(t *testing.T, body string) []ast.Stmt {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", "package p\nfunc f() {\n"+body+"\n}\n", 0)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	decl := file.Decls[0].(*ast.FuncDecl)
	return decl.Body.List
}

func compileBlockForTest(t *testing.T, globals map[string]VarRef, body string) *Block {
	t.Helper()

	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(globals, systemInterface)
	block := compiler.AllocateBlock()
	if err := compiler.CompileBlock(block, parseStatements(t, body)); err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	return block
}

func compileProgramForTest(t *testing.T, globals map[string]VarRef, body string) (*Compiler, *Block) {
	t.Helper()

	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(globals, systemInterface)
	block := compiler.AllocateBlock()
	if err := compiler.CompileBlock(block, parseStatements(t, body)); err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	return compiler, block
}

func loadProgramIntoVM(vm *VM, compiler *Compiler) {
	for id, compiledBlock := range compiler.blocks {
		vm.Blocks[id] = VMBlock{ID: compiledBlock.ID, Scope: compiledBlock.Scope, InheritedLocals: compiledBlock.InheritedLocals, LocalCount: compiledBlock.LocalCount, Bytes: compiledBlock.Bytes, Consts: compiledBlock.Consts}
	}
}
