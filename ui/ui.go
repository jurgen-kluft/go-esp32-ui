package ui

import (
	"github.com/jurgen-kluft/go-gui-app/imgui"
)

// ============================================================================
// 1. GLOBAL STATE TABLE & HARDWARE REGISTERS (Compilable & Modifiable on Mac)
// ============================================================================

// Local memory emulation mirroring the ESP32's State Table.
// Your menu layout reads and modifies these directly while running on the Mac.
var (
	UiMode           int32 = 0
	Light1Status     int32 = 0
	Light2Status     int32 = 0
	Light1Brightness int32 = 0

	// Read-Only Hardware Input Registers (Fed continuously by the ImGui Mouse engine)
	Finger0State int32 = 0 // 0 = Released, 1 = Pressed
	Finger1State int32 = 0 // 0 = Released, 1 = Pressed
	Finger0X     int32 = 0
	Finger0Y     int32 = 0
	Finger1X     int32 = 0
	Finger1Y     int32 = 0
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
	FontMain  string = "inter_24"
	FontSmall string = "inter_16"
)

// UI Menu State Modes
const (
	ModeStandardGrid   int32 = 0
	ModeDimmerOverlay  int32 = 1
	ModeFloorOverview  int32 = 2
	ModeRainingOverlay int32 = 3
)

// Gesture Bitmask Filters
const (
	GestureTap       byte = 0x01
	GestureHold      byte = 0x02
	GestureSlide     byte = 0x03
	GestureDoubleTap byte = 0x04
)

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
func DrawSprite(x, y int, path string) {
	drawList := imgui.WindowDrawList()
	canvasPos := imgui.CursorScreenPos()

	// Compute relative positions in the simulator box
	pMin := imgui.Vec2{X: canvasPos.X + float32(x), Y: canvasPos.Y + float32(y)}

	// For simulation scaffolding, we use standard dimensions based on assets
	w, h := float32(120), float32(120)
	if path == "bg/charcoal_top_banner.png" {
		w, h = 480, 50
	} else if path == "overlays/dim_backdrop.png" {
		w, h = 480, 480
	} else if path == "overlays/slider_card_360x280.png" {
		w, h = 360, 280
	} else if path == "overlays/alert_rain_fullscreen.png" {
		w, h = 480, 480
	} else if path == "shapes/circle_red_50x50.png" {
		w, h = 50, 50
	}

	pMax := imgui.Vec2{X: pMin.X + w, Y: pMin.Y + h}

	// Choose box color highlights to look representative
	var bgImguiColor uint32
	if path == "overlays/dim_backdrop.png" {
		bgImguiColor = packedColorFromRGBA(0, 0, 0, 150) // Semi-transparent fade
	} else if path == "overlays/slider_card_360x280.png" {
		bgImguiColor = packedColorFromRGBA(240, 240, 245, 255) // Card stock white
	} else if path == "buttons/btn_gold_on_120x120.png" {
		bgImguiColor = packedColorFromRGBA(212, 175, 55, 255) // Active Gold
	} else {
		bgImguiColor = packedColorFromRGBA(70, 70, 75, 255) // Component Dark Grey
	}

	drawList.AddRectFilledV(pMin, pMax, bgImguiColor, 6.0, imgui.DrawFlagsRoundCornersAll)
}

// DrawText prints layout typography strings into the ImGui viewport window
func DrawText(x, y int, font string, colorVal Color, text string) {
	drawList := imgui.WindowDrawList()
	canvasPos := imgui.CursorScreenPos()
	pos := imgui.Vec2{X: canvasPos.X + float32(x), Y: canvasPos.Y + float32(y)}

	fontPtr := imgui.CurrentFont()
	fontSize := fontPtr.LegacySize()

	r, g, b := rgb565ToRGBA(colorVal)
	drawList.AddTextFontPtr(fontPtr, fontSize, pos, packedColorFromRGBA(r, g, b, 255), text)
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

func CheckTimer(timerName string) bool {
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
		Finger0X = int32(relX)
		Finger0Y = int32(relY)
		if imgui.IsMouseDown(imgui.MouseButtonLeft) {
			Finger0State = 1
		} else {
			Finger0State = 0
		}
	} else {
		// Finger 1 Mode (Shift Key Held): Locks Finger 0 in place
		// and processes second tracking stream inside Finger 1 registers instead!
		Finger1X = int32(relX)
		Finger1Y = int32(relY)
		if imgui.IsMouseDown(imgui.MouseButtonLeft) {
			Finger1State = 1
		} else {
			Finger1State = 0
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
	if imgui.IsWindowHovered() && Finger0State == 1 && !Env.IsShiftHeld {
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

	if Finger0State == 1 {
		f0Vec := imgui.Vec2{X: canvasPos.X + float32(Finger0X), Y: canvasPos.Y + float32(Finger0Y)}
		drawList.AddCircleFilled(f0Vec, 10.0, packedColorFromRGBA(46, 204, 113, 200)) // Green crosshair
		drawList.AddTextVec2(f0Vec, packedColorFromRGBA(0, 0, 0, 255), "F0")
	}
	if Finger1State == 1 {
		f1Vec := imgui.Vec2{X: canvasPos.X + float32(Finger1X), Y: canvasPos.Y + float32(Finger1Y)}
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
	targetX := int(Finger0X)
	targetY := int(Finger0Y)
	if Env.LastGestureFired == GestureSlide && Env.IsShiftHeld {
		targetX = int(Finger1X)
		targetY = int(Finger1Y)
	}

	for _, zone := range Env.ActiveZones {
		if targetX >= int(zone.X) && targetX <= int(zone.X+zone.W) &&
			targetY >= int(zone.Y) && targetY <= int(zone.Y+zone.H) {
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
