package ui

import (
	"github.com/jurgen-kluft/go-esp32-ui/vm"
	"github.com/jurgen-kluft/go-gui-app/imgui"
)

// ============================================================================
// 1. GLOBAL STATE TABLE & HARDWARE REGISTERS (Compilable & Modifiable on Mac)
// ============================================================================

// Local memory emulation mirroring the ESP32's State Table.
// Your menu layout reads and modifies these directly while running on the Mac.

var (
	UIMode = vm.Var{Index: 0, Type: vm.VarTypeU8, Value: 0}

	DateString = vm.Var{Index: 4, Type: vm.VarTypeStr, Value: ""}
	TimeString = vm.Var{Index: 5, Type: vm.VarTypeStr, Value: ""}

	// Input variables for touch screen
	Finger0State = vm.Var{Index: 6, Type: vm.VarTypeU8, Value: 0}
	Finger1State = vm.Var{Index: 7, Type: vm.VarTypeU8, Value: 0}
	Finger0X     = vm.Var{Index: 8, Type: vm.VarTypeU16, Value: 0}
	Finger0Y     = vm.Var{Index: 9, Type: vm.VarTypeU16, Value: 0}
	Finger1X     = vm.Var{Index: 10, Type: vm.VarTypeU16, Value: 0}
	Finger1Y     = vm.Var{Index: 11, Type: vm.VarTypeU16, Value: 0}

	GroundfloorBathroomCeilingLight_State      = vm.Var{Index: 0, Type: vm.VarTypeU8, Value: 0}
	GroundfloorBathroomCeilingLight_Brightness = vm.Var{Index: 1, Type: vm.VarTypeU8, Value: 0}
	GroundfloorBathroomMirrorLight_State       = vm.Var{Index: 2, Type: vm.VarTypeU8, Value: 0}
	GroundfloorBathroomMirrorLight_Brightness  = vm.Var{Index: 3, Type: vm.VarTypeU8, Value: 0}

	// Sprites
	BgCharcoalTopBanner = vm.Var{Index: 11, Type: vm.VarTypeU32, Value: 0}

	BtnGoldOn120x120  = vm.Var{Index: 12, Type: vm.VarTypeU32, Value: 0}
	BtnGoldOff120x120 = vm.Var{Index: 13, Type: vm.VarTypeU32, Value: 0}
	BtnGreyOff120x120 = vm.Var{Index: 14, Type: vm.VarTypeU32, Value: 0}

	OverlayDimBackdrop       = vm.Var{Index: 15, Type: vm.VarTypeU32, Value: 0}
	OverlaySliderCard360x280 = vm.Var{Index: 16, Type: vm.VarTypeU32, Value: 0}
	OverlayRainFullscreen    = vm.Var{Index: 17, Type: vm.VarTypeU32, Value: 0}
)

// ============================================================================
// 2. EMBEDDED ENGINE SYSTEM CONSTANTS (Used by Compiler and VM)
// ============================================================================

type Color uint16

const (
	ColorCharcoal Color = 0x18C3 // RGB565 dark slate
	ColorWhite    Color = 0xFFFF
	ColorBlack    Color = 0x0000
	ColorDarkGrey Color = 0x4208
	ColorRed      Color = 0xF800
)

const (
	FontMain  uint32 = 0
	FontSmall uint32 = 1
)

// UI Menu State Modes
var (
	ModeStandardGrid   = 0
	ModeDimmerOverlay  = 1
	ModeFloorOverview  = 2
	ModeRainingOverlay = 3
)

// Gesture Bitmask Filters
const (
	GestureTap       byte = 0x01
	GestureHold      byte = 0x02
	GestureSlide     byte = 0x03
	GestureDoubleTap byte = 0x04
)

// ============================================================================
// 3. SYSTEM CALL INTERFACE (Compiler & VM)
// ============================================================================

func DrawBackground(imageID vm.Var) {

}

func SetLightOnOff(lightID vm.Var, onOff uint32) {

}

func IsLightOn(lightID vm.Var) bool {
	return false
}

func SetLightBrightness(lightID vm.Var, brightness uint32) {

}

func GetLightBrightness(lightID vm.Var) uint32 {
	return 0
}

func SetLightColor(lightID vm.Var, color uint32) {

}

func GetLightColor(lightID vm.Var) uint32 {
	return 0
}

// ============================================================================
// 3. SIMULATOR ENVIRONMENT ENGINE
// ============================================================================

type ZoneDef struct {
	X, Y, W, H uint16
	Gesture    byte
	Action     func()
}

type SimulatorEnv struct {
	ActiveZones      []ZoneDef
	IsShiftHeld      bool
	LastGestureFired byte
	ClickTimer       float32 // Tracks time between clicks for Double-Tap logic
	ClickCount       int
}

// Global active instance of our mock simulator environment
var Env = &SimulatorEnv{
	ActiveZones: make([]ZoneDef, 0),
}

// RGB565 short code color parsing engine utility matching standard 16-bit rules
func rgb565ToRGBA(colorVal Color) (uint8, uint8, uint8) {
	r := uint8((colorVal >> 11) & 0x1F)
	g := uint8((colorVal >> 5) & 0x3F)
	b := uint8(colorVal & 0x1F)
	return (r * 255) / 31, (g * 255) / 63, (b * 255) / 31
}

func packedColorFromRGBA(r, g, b, a uint8) uint32 {
	return uint32(a)<<24 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
}

// ============================================================================
// 4. DRAWING & INTERACTION PRIMITIVES (ImGui Mockups)
// ============================================================================

// ClearScreen floods the 480x480 panel boundary with a background color
func ClearScreen(colorVal Color) {
	drawList := imgui.WindowDrawList()
	canvasPos := imgui.CursorScreenPos()
	panelMax := imgui.Vec2{X: canvasPos.X + 480, Y: canvasPos.Y + 480}

	r, g, b := rgb565ToRGBA(colorVal)
	drawList.AddRectFilledV(canvasPos, panelMax, packedColorFromRGBA(r, g, b, 255), 0.0, imgui.DrawFlagsRoundCornersNone)
}

// DrawSprite simulates a raw memory blit by rendering an overlay block inside ImGui
func DrawSprite(x, y int32, path vm.Var) {
	drawList := imgui.WindowDrawList()
	canvasPos := imgui.CursorScreenPos()

	// Compute relative positions in the simulator box
	pMin := imgui.Vec2{X: canvasPos.X + float32(x), Y: canvasPos.Y + float32(y)}

	// For simulation scaffolding, we use standard dimensions based on assets
	w, h := float32(120), float32(120)
	// if path == "bg/charcoal_top_banner.png" {
	// 	w, h = 480, 50
	// } else if path == "overlays/dim_backdrop.png" {
	// 	w, h = 480, 480
	// } else if path == "overlays/slider_card_360x280.png" {
	// 	w, h = 360, 280
	// } else if path == "overlays/alert_rain_fullscreen.png" {
	// 	w, h = 480, 480
	// } else if path == "shapes/circle_red_50x50.png" {
	// 	w, h = 50, 50
	// }

	pMax := imgui.Vec2{X: pMin.X + w, Y: pMin.Y + h}

	// Choose box color highlights to look representative
	var bgImguiColor uint32
	// if path == "overlays/dim_backdrop.png" {
	// 	bgImguiColor = packedColorFromRGBA(0, 0, 0, 150) // Semi-transparent fade
	// } else if path == "overlays/slider_card_360x280.png" {
	// 	bgImguiColor = packedColorFromRGBA(240, 240, 245, 255) // Card stock white
	// } else if path == "buttons/btn_gold_on_120x120.png" {
	// 	bgImguiColor = packedColorFromRGBA(212, 175, 55, 255) // Active Gold
	// } else {
	// 	bgImguiColor = packedColorFromRGBA(70, 70, 75, 255) // Component Dark Grey
	// }

	drawList.AddRectFilledV(pMin, pMax, bgImguiColor, 6.0, imgui.DrawFlagsRoundCornersAll)
}

// DrawText prints layout typography strings into the ImGui viewport window
func DrawText(font uint32, text string, x, y int32, colorVal Color) {
	drawList := imgui.WindowDrawList()
	canvasPos := imgui.CursorScreenPos()
	pos := imgui.Vec2{X: canvasPos.X + float32(x), Y: canvasPos.Y + float32(y)}

	fontPtr := imgui.CurrentFont()
	fontSize := fontPtr.LegacySize()

	r, g, b := rgb565ToRGBA(colorVal)
	drawList.AddTextFontPtr(fontPtr, fontSize, pos, packedColorFromRGBA(r, g, b, 255), text)
}

func DrawVar(font uint32, v vm.Var, x, y int32, colorVal Color) {
	drawList := imgui.WindowDrawList()
	canvasPos := imgui.CursorScreenPos()
	pos := imgui.Vec2{X: canvasPos.X + float32(x), Y: canvasPos.Y + float32(y)}

	fontPtr := imgui.CurrentFont()
	fontSize := fontPtr.LegacySize()

	if v.Type == vm.VarTypeStr {
		str := v.Value.(string)
		r, g, b := rgb565ToRGBA(colorVal)
		drawList.AddTextFontPtr(fontPtr, fontSize, pos, packedColorFromRGBA(r, g, b, 255), str)
	}
}

// ============================================================================
// Timers
// The virtual machine needs to update the active timers on each tick, so we
// provide a simple interface to manage them in the simulator environment.
// ============================================================================
const (
	maxTimers = 255
)

var (
	timerDurations [maxTimers]int32
	timerActive    [maxTimers]bool

	// Track assigned named targets dynamically during simulation execution
	timerNameMap   = make(map[string]int)
	nextSimTimerID = 0
)

// resolveSimTimerID maps a string name to a 0-255 hardware tracking register slot index
func resolveSimTimerID(name string) int {
	id, exists := timerNameMap[name]
	if !exists {
		if nextSimTimerID >= maxTimers {
			panic("Simulator Fault: Exceeded maximum concurrent hardware timers pool limit (255 slots)")
		}
		id = nextSimTimerID
		timerNameMap[name] = id
		nextSimTimerID++
	}
	return id
}

func StartTimer(timerName string, durationMs int) {
	id := resolveSimTimerID(timerName)
	timerDurations[id] = int32(durationMs)
	timerActive[id] = true
}

func StopTimer(timerName string) {
	id := resolveSimTimerID(timerName)
	timerActive[id] = false
	timerDurations[id] = 0
}

func GetTimer(timerName string) bool {
	id := resolveSimTimerID(timerName)
	if !timerActive[id] {
		return false
	}
	if timerDurations[id] <= 0 {
		timerActive[id] = false
		return true
	}
	return false
}

// ============================================================================
// 7. Event Zone registration and gesture evaluation
// ============================================================================

// RegisterZone stores interaction nodes in the active tracking frame queue
func RegisterZone(x, y, w, h int, gesture byte, action func()) {
	Env.ActiveZones = append(Env.ActiveZones, ZoneDef{
		X: uint16(x), Y: uint16(y), W: uint16(w), H: uint16(h),
		Gesture: gesture,
		Action:  action,
	})
}

// ============================================================================
// 5. WINDOW FRAME CONTEXT PIPELINE (Main Loop Engine)
// ============================================================================

// RenderSimulationWindow manages layout frame ticks, emulates multi-touch via mouse,
// and evaluates gestures in real time on the Mac Mini screen.
func RenderSimulationWindow(renderLayoutBlock func()) {
	imgui.SetNextWindowSizeV(imgui.Vec2{X: 520, Y: 580}, imgui.CondAlways)
	imgui.Begin("ESP32-S3 ST7701 Panel Workspace")

	// 1. Capture Mac Key Modifiers (Left Shift / Right Shift)
	Env.IsShiftHeld = imgui.IsKeyDown(imgui.KeyLeftShift) || imgui.IsKeyDown(imgui.KeyRightShift)

	canvasPos := imgui.CursorScreenPos()
	mousePos := imgui.MousePos()

	// Compute active relative touch vectors inside display coordinates
	relX := int(mousePos.X - canvasPos.X)
	relY := int(mousePos.Y - canvasPos.Y)

	// 2. Core Multi-Touch Intercept Emulation Math
	if !Env.IsShiftHeld {
		// Finger 0 Mode: Left Mouse click acts as main finger
		Finger0X.Value = int32(relX)
		Finger0Y.Value = int32(relY)
		if imgui.IsMouseDown(imgui.MouseButtonLeft) {
			Finger0State.Value = 1
		} else {
			Finger0State.Value = 0
		}
	} else {
		// Finger 1 Mode (Shift Key Held): Locks Finger 0 in place
		// and processes second tracking stream inside Finger 1 registers instead!
		Finger1X.Value = int32(relX)
		Finger1Y.Value = int32(relY)
		if imgui.IsMouseDown(imgui.MouseButtonLeft) {
			Finger1State.Value = 1
		} else {
			Finger1State.Value = 0
		}
	}

	// 3. Gesture Detection Extraction (Tap, Double-Tap, and Drag/Slide)
	io := imgui.CurrentIO()
	deltaTime := io.DeltaTime()

	if Env.ClickCount > 0 {
		Env.ClickTimer += deltaTime
		if Env.ClickTimer > 0.3 { // Reset window after 300ms
			if Env.ClickCount == 1 {
				Env.LastGestureFired = GestureTap
			}
			Env.ClickCount = 0
			Env.ClickTimer = 0
		}
	}

	if imgui.IsMouseClickedBoolV(imgui.MouseButtonLeft, false) {
		Env.ClickCount++
		Env.ClickTimer = 0
		if Env.ClickCount == 2 {
			Env.LastGestureFired = GestureDoubleTap
			Env.ClickCount = 0
		}
	}

	// If moving mouse while clicking down inside a zone boundary, treat it as a Slide gesture
	if imgui.IsMouseDraggingV(imgui.MouseButtonLeft, 1.0) {
		Env.LastGestureFired = GestureSlide
	}

	// If a finger has stayed down without heavy panning movement, flag a long press Hold trigger
	if imgui.IsWindowHovered() && Finger0State.Value == 1 && !Env.IsShiftHeld {
		// Basic fallback simulation shortcut: Clicking and holding right-mouse button
		// can also act as an immediate shortcut for a long-press GestureHold event.
		if imgui.IsMouseClickedBoolV(imgui.MouseButtonRight, false) || io.MouseDownDuration()[0] > 0.5 {
			Env.LastGestureFired = GestureHold
		}
	}

	// 4. Render Layout & Process Touch Hits
	// Clear the local boundary registry cache stack
	Env.ActiveZones = Env.ActiveZones[:0]

	// Execute your layout function (e.g. RenderBathroomPage())
	renderLayoutBlock()

	// Walk the active interaction zones to match clicks
	evaluateActiveTouchGestures()

	// 5. Visual Overlays: Render crosshair nodes for tracking points
	drawList := imgui.WindowDrawList()
	panelMax := imgui.Vec2{X: canvasPos.X + 480, Y: canvasPos.Y + 480}
	drawList.AddRect(canvasPos, panelMax, packedColorFromRGBA(255, 255, 255, 50)) // Border frame boundary

	if Finger0State.Value == 1 {
		f0Vec := imgui.Vec2{X: canvasPos.X + Finger0X.AsFloat32(), Y: canvasPos.Y + Finger0Y.AsFloat32()}
		drawList.AddCircleFilled(f0Vec, 10.0, packedColorFromRGBA(46, 204, 113, 200)) // Green crosshair
		drawList.AddTextVec2(f0Vec, packedColorFromRGBA(0, 0, 0, 255), "F0")
	}
	if Finger1State.Value == 1 {
		f1Vec := imgui.Vec2{X: canvasPos.X + Finger1X.AsFloat32(), Y: canvasPos.Y + Finger1Y.AsFloat32()}
		drawList.AddCircleFilled(f1Vec, 10.0, packedColorFromRGBA(52, 152, 219, 200)) // Blue crosshair
		drawList.AddTextVec2(f1Vec, packedColorFromRGBA(0, 0, 0, 255), "F1")
	}

	imgui.End()
}

func evaluateActiveTouchGestures() {
	if Env.LastGestureFired == 0x00 {
		return
	}

	// Choose which tracking stream point maps to current check targets
	targetX := Finger0X.AsInt32()
	targetY := Finger0Y.AsInt32()
	if Env.LastGestureFired == GestureSlide && Env.IsShiftHeld {
		targetX = Finger1X.AsInt32()
		targetY = Finger1Y.AsInt32()
	}

	for _, zone := range Env.ActiveZones {
		if targetX >= int32(zone.X) && targetX <= int32(zone.X+zone.W) &&
			targetY >= int32(zone.Y) && targetY <= int32(zone.Y+zone.H) {
			if zone.Gesture == Env.LastGestureFired {
				zone.Action()

				// Keep slide stream open across frames; consumption resets transient discrete actions
				if Env.LastGestureFired != GestureSlide {
					Env.LastGestureFired = 0x00
				}
				return
			}
		}
	}

	// Reset if click drops down outside zones completely
	if !imgui.IsMouseDown(imgui.MouseButtonLeft) {
		Env.LastGestureFired = 0x00
	}
}
