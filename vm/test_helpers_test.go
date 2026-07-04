package vm

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

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

func compileBlockForTest(t *testing.T, globals map[string]ID, body string) *Block {
	t.Helper()

	systemInterface := NewCompilerSystemCalls()
	compiler := NewCompiler(globals, systemInterface)
	block := compiler.AllocateBlock()
	if err := compiler.CompileBlock(block, parseStatements(t, body)); err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	return block
}

func compileProgramForTest(t *testing.T, globals map[string]ID, body string) (*Compiler, *Block) {
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
		vm.Blocks[id] = VMBlock{ID: compiledBlock.ID, LocalCount: compiledBlock.LocalCount, Bytes: compiledBlock.Bytes}
	}
}
