package vm

import "testing"

func TestVarHasFlag(t *testing.T) {
	variable := Var{Flags: VarFlagConst | VarFlagPtr}

	if !variable.HasFlag(VarFlagConst) {
		t.Fatal("expected const flag to be set")
	}
	if !variable.HasFlag(VarFlagPtr) {
		t.Fatal("expected ptr flag to be set")
	}
	if variable.HasFlag(VarFlagNone) {
		t.Fatal("zero flag should not be reported as set")
	}
}
