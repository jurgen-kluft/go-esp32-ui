package ui

import (
	"github.com/jurgen-kluft/go-esp32-ui/vm"
)

func VarToUint32(v vm.Var) uint32 {
	return v.AsUint32()
}

func VarToInt32(v vm.Var) int32 {
	return v.AsInt32()
}

func VarToFloat32(v vm.Var) float32 {
	return v.AsFloat32()
}

func VarAssign[T any](dst *vm.Var, value T) {
	if dst == nil {
		return
	}
	assignValueToVar(dst, any(value))
}

func VarEq[T any](left vm.Var, right T) bool {
	return compareVarIntrinsic(left, any(right), compareEq)
}

func VarLt[T any](left vm.Var, right T) bool {
	return compareVarIntrinsic(left, any(right), compareLt)
}

func VarLe[T any](left vm.Var, right T) bool {
	return compareVarIntrinsic(left, any(right), compareLe)
}

func VarGt[T any](left vm.Var, right T) bool {
	return compareVarIntrinsic(left, any(right), compareGt)
}

func VarGe[T any](left vm.Var, right T) bool {
	return compareVarIntrinsic(left, any(right), compareGe)
}

type compareMode uint8

const (
	compareEq compareMode = iota
	compareLt
	compareLe
	compareGt
	compareGe
)

func compareVarIntrinsic(left vm.Var, right any, mode compareMode) bool {
	switch typed := right.(type) {
	case vm.Var:
		return compareVars(left, typed, mode)
	case *vm.Var:
		if typed == nil {
			return false
		}
		return compareVars(left, *typed, mode)
	case string:
		if mode != compareEq {
			return false
		}
		return left.AsString() == typed
	case bool:
		boolToUint32 := func(value bool) uint32 {
			if value {
				return 1
			}
			return 0
		}
		return compareUint32(left.AsUint32(), boolToUint32(typed), mode)
	case float32:
		return compareFloat32(left.AsFloat32(), typed, mode)
	case float64:
		return compareFloat32(left.AsFloat32(), float32(typed), mode)
	case int:
		return compareInt32(left.AsInt32(), int32(typed), mode)
	case int8:
		return compareInt32(left.AsInt32(), int32(typed), mode)
	case int16:
		return compareInt32(left.AsInt32(), int32(typed), mode)
	case int32:
		return compareInt32(left.AsInt32(), typed, mode)
	case uint:
		return compareUint32(left.AsUint32(), uint32(typed), mode)
	case uint8:
		return compareUint32(left.AsUint32(), uint32(typed), mode)
	case uint16:
		return compareUint32(left.AsUint32(), uint32(typed), mode)
	case uint32:
		return compareUint32(left.AsUint32(), typed, mode)
	default:
		return false
	}
}

func compareVars(left, right vm.Var, mode compareMode) bool {
	if left.Type == vm.VarTypeStr || right.Type == vm.VarTypeStr {
		if mode != compareEq {
			return false
		}
		return left.AsString() == right.AsString()
	}
	if left.Type == vm.VarTypeF32 || right.Type == vm.VarTypeF32 {
		return compareFloat32(left.AsFloat32(), right.AsFloat32(), mode)
	}
	if left.Type == vm.VarTypeS8 || left.Type == vm.VarTypeS16 || left.Type == vm.VarTypeS32 ||
		right.Type == vm.VarTypeS8 || right.Type == vm.VarTypeS16 || right.Type == vm.VarTypeS32 {
		return compareInt32(left.AsInt32(), right.AsInt32(), mode)
	}
	return compareUint32(left.AsUint32(), right.AsUint32(), mode)
}

func compareUint32(left, right uint32, mode compareMode) bool {
	switch mode {
	case compareEq:
		return left == right
	case compareLt:
		return left < right
	case compareLe:
		return left <= right
	case compareGt:
		return left > right
	case compareGe:
		return left >= right
	default:
		return false
	}
}

func compareInt32(left, right int32, mode compareMode) bool {
	switch mode {
	case compareEq:
		return left == right
	case compareLt:
		return left < right
	case compareLe:
		return left <= right
	case compareGt:
		return left > right
	case compareGe:
		return left >= right
	default:
		return false
	}
}

func compareFloat32(left, right float32, mode compareMode) bool {
	switch mode {
	case compareEq:
		return left == right
	case compareLt:
		return left < right
	case compareLe:
		return left <= right
	case compareGt:
		return left > right
	case compareGe:
		return left >= right
	default:
		return false
	}
}

func assignValueToVar(dst *vm.Var, value any) {
	switch typed := value.(type) {
	case vm.Var:
		assignFromVar(dst, typed)
	case *vm.Var:
		if typed != nil {
			assignFromVar(dst, *typed)
		}
	case string:
		dst.Value = typed
	case bool:
		dst.Value = typed
	case float32:
		if dst.Type == vm.VarTypeF32 {
			dst.Value = typed
		} else {
			dst.SetUint32Value(uint32(typed))
		}
	case float64:
		if dst.Type == vm.VarTypeF32 {
			dst.Value = float32(typed)
		} else {
			dst.SetUint32Value(uint32(typed))
		}
	case int:
		dst.SetUint32Value(uint32(int32(typed)))
	case int8:
		dst.SetUint32Value(uint32(int32(typed)))
	case int16:
		dst.SetUint32Value(uint32(int32(typed)))
	case int32:
		dst.SetUint32Value(uint32(typed))
	case uint:
		dst.SetUint32Value(uint32(typed))
	case uint8:
		dst.SetUint32Value(uint32(typed))
	case uint16:
		dst.SetUint32Value(uint32(typed))
	case uint32:
		dst.SetUint32Value(typed)
	}
}

func assignFromVar(dst *vm.Var, src vm.Var) {
	if src.Type == vm.VarTypeStr {
		dst.Value = src.AsString()
		return
	}
	if src.Type == vm.VarTypeF32 {
		if dst.Type == vm.VarTypeF32 {
			dst.Value = src.AsFloat32()
		} else {
			dst.SetUint32Value(uint32(src.AsFloat32()))
		}
		return
	}
	if src.Type == vm.VarTypeBool {
		dst.Value = src.AsUint32() != 0
		return
	}
	dst.SetUint32Value(src.Uint32Value())
}
