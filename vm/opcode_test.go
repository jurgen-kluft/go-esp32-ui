package vm

import "testing"

func TestPackUnpackVarRef(t *testing.T) {
	testCases := []VarRef{
		GlobalRef(0),
		ConstRef(varRefIndexMask),
		LocalRef(1),
	}

	for _, testCase := range testCases {
		raw := testCase.Pack()
		decoded := UnpackVarRef(raw)
		if decoded != testCase {
			t.Fatalf("round-trip mismatch: got %+v want %+v", decoded, testCase)
		}
	}
}