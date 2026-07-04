package vm

import "testing"

func TestPackUnpackID(t *testing.T) {
	testCases := []ID{
		{Type: 1, Idx: 0},
		{Type: 7, Idx: idIndexMask},
		{Type: 255, Idx: 1},
	}

	for _, testCase := range testCases {
		raw := testCase.Pack()
		decoded := NewID(raw)
		if decoded != testCase {
			t.Fatalf("round-trip mismatch: got %+v want %+v", decoded, testCase)
		}
	}
}