package vm

type SystemCallID uint8

const (
	SystemCallDrawBackground       SystemCallID = 1
	SystemCallDrawSprite           SystemCallID = 2
	SystemCallDrawText             SystemCallID = 3
	SystemCallDrawVar              SystemCallID = 4
	SystemCallStartTimer           SystemCallID = 5
	SystemCallStopTimer            SystemCallID = 6
	SystemCallIsTimerDone          SystemCallID = 7
	SystemCallSetLightOnOff        SystemCallID = 8
	SystemCallIsLightOn            SystemCallID = 9
	SystemCallSetLightBrightness   SystemCallID = 10
	SystemCallGetLightBrightness   SystemCallID = 11
	SystemCallSetLightColor        SystemCallID = 12
	SystemCallGetLightColor        SystemCallID = 13
	SystemCallRegisterZone         SystemCallID = 14
	SystemCallTurnRelayOnOff       SystemCallID = 15
	SystemCallSetDisplayBrightness SystemCallID = 16
)

type VmSystemInterface interface {
	DrawBackground(imageID uint32)
	DrawSprite(spriteID, x, y uint32)
	DrawText(fontID uint32, text string, x, y, color uint32)
	DrawVar(fontID uint32, _var uint32, x, y, color uint32)

	StartTimer(timerID, duration uint32)
	StopTimer(timerID uint32)
	IsTimerDone(timerID uint32) bool

	SetLightOnOff(lightID, onOff uint32)
	IsLightOn(lightID uint32) bool
	SetLightBrightness(lightID, brightness uint32)
	GetLightBrightness(lightID uint32) uint32
	SetLightColor(lightID, color uint32)
	GetLightColor(lightID uint32) uint32

	RegisterZone(zoneID uint32, x, y, width, height uint32, gesture uint8) bool

	TurnRelayOnOff(relay int8, status int8)
	SetDisplayBrightness(brightness uint8)
}

func NewCompilerSystemCalls() map[string]uint8 {
	systemCallMap := make(map[string]uint8)

	systemCallMap["DrawBackground"] = uint8(SystemCallDrawBackground)
	systemCallMap["DrawSprite"] = uint8(SystemCallDrawSprite)
	systemCallMap["DrawText"] = uint8(SystemCallDrawText)
	systemCallMap["DrawVar"] = uint8(SystemCallDrawVar)

	systemCallMap["StartTimer"] = uint8(SystemCallStartTimer)
	systemCallMap["StopTimer"] = uint8(SystemCallStopTimer)
	systemCallMap["IsTimerDone"] = uint8(SystemCallIsTimerDone)

	systemCallMap["SetLightOnOff"] = uint8(SystemCallSetLightOnOff)
	systemCallMap["IsLightOn"] = uint8(SystemCallIsLightOn)
	systemCallMap["SetLightBrightness"] = uint8(SystemCallSetLightBrightness)
	systemCallMap["GetLightBrightness"] = uint8(SystemCallGetLightBrightness)
	systemCallMap["SetLightColor"] = uint8(SystemCallSetLightColor)
	systemCallMap["GetLightColor"] = uint8(SystemCallGetLightColor)

	systemCallMap["RegisterZone"] = uint8(SystemCallRegisterZone)

	systemCallMap["TurnRelayOnOff"] = uint8(SystemCallTurnRelayOnOff)
	systemCallMap["SetDisplayBrightness"] = uint8(SystemCallSetDisplayBrightness)

	return systemCallMap
}
