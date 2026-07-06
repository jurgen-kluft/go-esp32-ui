package vm

import "testing"

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