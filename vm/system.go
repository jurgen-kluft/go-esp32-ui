package vm

type SystemCallID uint8

const (
	SystemCallDrawBackground     SystemCallID = 1
	SystemCallDrawSprite         SystemCallID = 2
	SystemCallDrawText           SystemCallID = 3
	SystemCallDrawVar            SystemCallID = 4
	SystemCallStartTimer         SystemCallID = 5
	SystemCallStopTimer          SystemCallID = 6
	SystemCallGetTimer           SystemCallID = 7
	SystemCallSetLightOnOff      SystemCallID = 8
	SystemCallIsLightOn          SystemCallID = 9
	SystemCallSetLightBrightness SystemCallID = 10
	SystemCallGetLightBrightness SystemCallID = 11
	SystemCallSetLightColor      SystemCallID = 12
	SystemCallGetLightColor      SystemCallID = 13
)

type VmSystemInterface interface {
	DrawBackground(imageID uint32)
	DrawSprite(spriteID, x, y uint32)
	DrawText(fontID, textID, x, y, color uint32)
	DrawVar(fontID, varID, x, y, color uint32)

	StartTimer(timerID, duration uint32)
	StopTimer(timerID uint32)
	GetTimer(timerID uint32) uint32

	SetLightOnOff(lightID, onOff uint32)
	IsLightOn(lightID uint32) uint32
	SetLightBrightness(lightID, brightness uint32)
	GetLightBrightness(lightID uint32) uint32
	SetLightColor(lightID, color uint32)
	GetLightColor(lightID uint32) uint32
}

type VmGlobalStateInterface interface {
	GetGlobalVar(id ID) (uint32, bool)
	SetGlobalVar(id ID, value uint32) bool
}

type CompilerSystemInterface interface {
	RegisterSystemCall(name string) (uint8, bool)
}

type CompilerSystemCalls struct {
	systemCallMap map[string]uint8
}

func NewCompilerSystemCalls() CompilerSystemInterface {
	csc := &CompilerSystemCalls{
		systemCallMap: make(map[string]uint8),
	}

	csc.systemCallMap["DrawBackground"] = uint8(SystemCallDrawBackground)
	csc.systemCallMap["DrawSprite"] = uint8(SystemCallDrawSprite)
	csc.systemCallMap["DrawText"] = uint8(SystemCallDrawText)
	csc.systemCallMap["DrawVar"] = uint8(SystemCallDrawVar)

	csc.systemCallMap["StartTimer"] = uint8(SystemCallStartTimer)
	csc.systemCallMap["StopTimer"] = uint8(SystemCallStopTimer)
	csc.systemCallMap["GetTimer"] = uint8(SystemCallGetTimer)

	csc.systemCallMap["SetLightOnOff"] = uint8(SystemCallSetLightOnOff)
	csc.systemCallMap["IsLightOn"] = uint8(SystemCallIsLightOn)
	csc.systemCallMap["SetLightBrightness"] = uint8(SystemCallSetLightBrightness)
	csc.systemCallMap["GetLightBrightness"] = uint8(SystemCallGetLightBrightness)
	csc.systemCallMap["SetLightColor"] = uint8(SystemCallSetLightColor)
	csc.systemCallMap["GetLightColor"] = uint8(SystemCallGetLightColor)

	return csc
}

func (csc *CompilerSystemCalls) RegisterSystemCall(name string) (uint8, bool) {
	id, ok := csc.systemCallMap[name]
	return id, ok
}
