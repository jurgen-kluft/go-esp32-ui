package vm

import (
	"math"
)

type ValueKind uint8

const (
	ValueKindU32 ValueKind = iota
	ValueKindS32
	ValueKindF32
)

type Value struct {
	Kind ValueKind
	Bits uint32
}

func (v Value) AsInt8() int8 {
	return int8(v.AsInt32())
}

func (v Value) AsInt16() int16 {
	return int16(v.AsInt32())
}

func (v Value) AsInt32() int32 {
	switch v.Kind {
	case ValueKindU32:
		return int32(v.Bits)
	case ValueKindF32:
		return int32(math.Float32frombits(v.Bits))
	default:
		return int32(v.Bits)
	}
}

func (v Value) AsUint8() uint8 {
	return uint8(v.AsUint32())
}

func (v Value) AsUint16() uint16 {
	return uint16(v.AsUint32())
}

func (v Value) AsUint32() uint32 {
	switch v.Kind {
	case ValueKindS32:
		return uint32(int32(v.Bits))
	case ValueKindF32:
		return uint32(math.Float32frombits(v.Bits))
	default:
		return v.Bits
	}
}

func (v Value) AsFloat32() float32 {
	switch v.Kind {
	case ValueKindU32:
		return float32(v.Bits)
	case ValueKindS32:
		return float32(int32(v.Bits))
	default:
		return math.Float32frombits(v.Bits)
	}
}

func (v *Value) SetInt8(value int8) {
	v.SetInt32(int32(value))
}

func (v *Value) SetInt16(value int16) {
	v.SetInt32(int32(value))
}

func (v *Value) SetInt32(value int32) {
	v.Kind = ValueKindS32
	v.Bits = uint32(value)
}

func (v *Value) SetUint8(value uint8) {
	v.SetUint32(uint32(value))
}

func (v *Value) SetUint16(value uint16) {
	v.SetUint32(uint32(value))
}

func (v *Value) SetUint32(value uint32) {
	v.Kind = ValueKindU32
	v.Bits = value
}

func (v *Value) SetFloat32(value float32) {
	v.Kind = ValueKindF32
	v.Bits = math.Float32bits(value)
}
