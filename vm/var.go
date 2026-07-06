package vm

import (
	"math"
)

type VarType uint8

const (
	VarTypeNone VarType = 0
	VarTypeU8   VarType = 1
	VarTypeU16  VarType = 2
	VarTypeU32  VarType = 3
	VarTypeS8   VarType = 4
	VarTypeS16  VarType = 5
	VarTypeS32  VarType = 6
	VarTypeF32  VarType = 7
	VarTypeStr  VarType = 8
	VarTypeBool VarType = 9
)

func isFloatVarType(varType VarType) bool {
	return varType == VarTypeF32
}

func isSignedVarType(varType VarType) bool {
	switch varType {
	case VarTypeS8, VarTypeS16, VarTypeS32:
		return true
	default:
		return false
	}
}

type VarFlag uint8

const (
	VarFlagNone  VarFlag = 0
	VarFlagConst VarFlag = 1 << 0
	VarFlagPtr   VarFlag = 1 << 1
	VarFlagTemp  VarFlag = 1 << 2
)

type Var struct {
	Generation uint32
	Index      uint16
	Type       VarType
	Flags      VarFlag
	Value      any
}

func (v Var) IsSet() bool {
	return v.Type != VarTypeNone
}

func (v Var) HasFlag(flag VarFlag) bool {
	return v.Flags&flag != 0
}

func (v Var) AsString() string {
	if str, ok := v.Value.(string); ok {
		return str
	}
	if str, ok := v.Value.(programImageString); ok {
		return string(str)
	}
	return "?"
}

func (v Var) Uint32Value() uint32 {
	switch value := v.Value.(type) {
	case uint8:
		return uint32(value)
	case uint16:
		return uint32(value)
	case uint32:
		return value
	case int8:
		return uint32(int32(value))
	case int16:
		return uint32(int32(value))
	case int32:
		return uint32(value)
	case float32:
		return math.Float32bits(value)
	case bool:
		if value {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func (v Var) AsUint32() uint32 {
	switch value := v.Value.(type) {
	case uint8:
		return uint32(value)
	case uint16:
		return uint32(value)
	case uint32:
		return value
	case int8:
		return uint32(int32(value))
	case int16:
		return uint32(int32(value))
	case int32:
		return uint32(value)
	case float32:
		return uint32(value)
	case bool:
		if value {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func (v Var) AsInt32() int32 {
	switch value := v.Value.(type) {
	case uint8:
		return int32(value)
	case uint16:
		return int32(value)
	case uint32:
		return int32(value)
	case int8:
		return int32(value)
	case int16:
		return int32(value)
	case int32:
		return value
	case float32:
		return int32(value)
	case bool:
		if value {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func (v Var) AsFloat32() float32 {
	switch value := v.Value.(type) {
	case uint8:
		return float32(value)
	case uint16:
		return float32(value)
	case uint32:
		return float32(value)
	case int8:
		return float32(value)
	case int16:
		return float32(value)
	case int32:
		return float32(value)
	case float32:
		return value
	case bool:
		if value {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func (v *Var) SetUint32Value(bits uint32) {
	switch v.Type {
	case VarTypeU8:
		v.Value = uint8(bits)
	case VarTypeU16:
		v.Value = uint16(bits)
	case VarTypeU32:
		v.Value = bits
	case VarTypeS8:
		v.Value = int8(int32(bits))
	case VarTypeS16:
		v.Value = int16(int32(bits))
	case VarTypeS32:
		v.Value = int32(bits)
	case VarTypeF32:
		v.Value = math.Float32frombits(bits)
	case VarTypeBool:
		v.Value = bits != 0
	default:
		v.Value = bits
	}
}

func (v *Var) Assign(value any) {
	if v == nil {
		return
	}

	switch typed := value.(type) {
	case Var:
		if typed.Type == VarTypeStr {
			v.Value = typed.AsString()
			return
		}
		if typed.Type == VarTypeBool {
			v.Value = typed.AsUint32() != 0
			return
		}
		v.SetUint32Value(typed.Uint32Value())
	case *Var:
		if typed != nil {
			v.Assign(*typed)
		}
	case string:
		v.Value = typed
	case bool:
		v.Value = typed
	case uint8:
		v.SetUint32Value(uint32(typed))
	case uint16:
		v.SetUint32Value(uint32(typed))
	case uint32:
		v.SetUint32Value(typed)
	case uint:
		v.SetUint32Value(uint32(typed))
	case int8:
		v.SetUint32Value(uint32(int32(typed)))
	case int16:
		v.SetUint32Value(uint32(int32(typed)))
	case int32:
		v.SetUint32Value(uint32(typed))
	case int:
		v.SetUint32Value(uint32(int32(typed)))
	case float32:
		if v.Type == VarTypeF32 {
			v.Value = typed
		} else {
			v.SetUint32Value(uint32(typed))
		}
	case float64:
		if v.Type == VarTypeF32 {
			v.Value = float32(typed)
		} else {
			v.SetUint32Value(uint32(typed))
		}
	}
}

const (
	GlobalRefType      uint8  = 0
	ConstRefType       uint8  = 1
	LocalRefType       uint8  = 2
	TempRefType        uint8  = 3
	varRefIndexMask    uint32 = 0x00FFFFFF
	varRefStorageShift uint32 = 24
)

type VarRef struct {
	Storage uint8
	Index   uint32
}

func GlobalRef(index uint32) VarRef {
	return VarRef{Storage: GlobalRefType, Index: index}
}

func ConstRef(index uint32) VarRef {
	return VarRef{Storage: ConstRefType, Index: index}
}

func LocalRef(index uint32) VarRef {
	return VarRef{Storage: LocalRefType, Index: index}
}

func TempRef(index uint32) VarRef {
	return VarRef{Storage: TempRefType, Index: index}
}

// StackRef is the runtime-facing reference shape for operand-stack entries.
// The bytecode still uses packed VarRef operands; this type carries the same
// storage identity explicitly plus a runtime validity token for arena-backed
// locals and temps so stale refs can be rejected after slot reuse.
type StackRef struct {
	Storage uint8
	Index   uint32
	ScopeID uint32
}

func StackRefFromVarRef(ref VarRef, scopeID uint32) StackRef {
	return StackRef{Storage: ref.Storage, Index: ref.Index, ScopeID: scopeID}
}

func (ref VarRef) Pack() uint32 {
	return (uint32(ref.Storage) << varRefStorageShift) | (ref.Index & varRefIndexMask)
}

func UnpackVarRef(raw uint32) VarRef {
	return VarRef{
		Storage: uint8(raw >> varRefStorageShift),
		Index:   raw & varRefIndexMask,
	}
}
