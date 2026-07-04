package pages

import (
	"fmt"

	. "github.com/jurgen-kluft/go-esp32-ui/ui"
)

var dateString string
var timeString string

var lightsOnOffMap = map[string]int8{
	"Light1": 0,
	"Light2": 0,
}

func LightIsOn(lightName string) bool {
	status, exists := lightsOnOffMap[lightName]
	if !exists {
		status = 0
	}
	return status == 1
}

func SetLightStatus(lightName string, status int8) {
	lightsOnOffMap[lightName] = status
}

var lightsBrightnessMap = map[string]uint8{
	"Light1": 0,
	"Light2": 0,
}

func GetLightBrightness(lightName string) uint8 {
	brightness, exists := lightsBrightnessMap[lightName]
	if !exists {
		brightness = 0
	}
	return brightness
}

func SetLightBrightness(lightName string, brightness uint8) {
	lightsBrightnessMap[lightName] = brightness
}

func Render1stFloorBathroomPage() {
	// 1. Establish a clean frame baseline by flooding the 480x480 canvas via hardware
	ClearScreen(ColorCharcoal)

	// 2. Global Top Border & Navigation Banner
	DrawSprite(0, 0, "bg/charcoal_top_banner.png")
	DrawText(20, 15, FontMain, ColorWhite, "Basement - Bathroom")

	// 3. STATE ROUTING LAYER A: Standard Operational Grid View
	if UiMode == ModeStandardGrid {

		// Define UI Grid positioning constraints natively in Go
		const startX = 40
		const startY = 100
		const btnSize = 120
		const spacing = 140

		// Small date on the left top corner of the grid
		DrawText(20, 60, FontSmall, ColorWhite, dateString)

		// Small time on the right top corner of the grid
		DrawText(400, 60, FontSmall, ColorWhite, timeString)

		// Grid Column 0, Row 0: Main Ceiling Light Button Container
		if LightIsOn("F1.Bathroom.CeilingLight") {
			DrawSprite(startX, startY, "buttons/btn_gold_on_120x120.png")
			DrawText(startX+15, startY+80, FontSmall, ColorWhite, "Ceiling (ON)")
		} else {
			DrawSprite(startX, startY, "buttons/btn_grey_off_120x120.png")
			DrawText(startX+15, startY+80, FontSmall, ColorWhite, "Ceiling (OFF)")
		}

		// Single-Tap: Instantly toggles the light status variable locally on the ESP32
		RegisterZone(startX, startY, btnSize, btnSize, GestureTap, func() {
			if LightIsOn("F1.Bathroom.CeilingLight") {
				SetLightStatus("F1.Bathroom.CeilingLight", 0)
			} else {
				SetLightStatus("F1.Bathroom.CeilingLight", 1)
			}
		})

		// Press & Hold: Hides the main grid and enters the advanced multi-touch dimmer overlay
		RegisterZone(startX, startY, btnSize, btnSize, GestureHold, func() {
			UiMode = ModeDimmerOverlay
		})

		// Grid Column 1, Row 0: Mirror Light Button Container
		if LightIsOn("F1.Bathroom.MirrorLight") {
			DrawSprite(startX+spacing, startY, "buttons/btn_gold_on_120x120.png")
			DrawText(startX+spacing+20, startY+80, FontSmall, ColorWhite, "Mirror (ON)")
		} else {
			DrawSprite(startX+spacing, startY, "buttons/btn_grey_off_120x120.png")
			DrawText(startX+spacing+20, startY+80, FontSmall, ColorWhite, "Mirror (OFF)")
		}

		RegisterZone(startX+spacing, startY, btnSize, btnSize, GestureTap, func() {
			if LightIsOn("F1.Bathroom.MirrorLight") {
				SetLightStatus("F1.Bathroom.MirrorLight", 0)
			} else {
				SetLightStatus("F1.Bathroom.MirrorLight", 1)
			}
		})

		// --- DOUBLE TAP SCREEN ESCAPE ACCELERATOR ---
		// A double-tap anywhere on the main body bounds returns the user to the Floor Overview layout
		RegisterZone(0, 50, 480, 430, GestureDoubleTap, func() {
			UiMode = ModeFloorOverview
		})
	}

	// 4. STATE ROUTING LAYER B: Advanced Multi-Touch Dimmer Overlay View
	if UiMode == ModeDimmerOverlay {
		// Blit a pre-rendered translucent black mask to fade back the underlying screen elements safely
		DrawSprite(0, 0, "overlays/dim_backdrop.png")

		// Render the modal overlay popup dialogue block frame
		DrawSprite(60, 100, "overlays/slider_card_360x280.png")
		DrawText(90, 130, FontMain, ColorBlack, "Ceiling Brightness")

		// Display current slider value via text tracking variable changes dynamically
		// (In a future step, our text compile pipeline will evaluate this variable)
		DrawText(90, 170, FontSmall, ColorDarkGrey, fmt.Sprintf("Intensity: %d%%", GetLightBrightness("F1.Bathroom.CeilingLight")))

		// --- ADVANCED SLIDER DRAG GESTURE ---
		// We track the slider bar boundaries. While Finger 1 slides horizontally inside it,
		// the AST compiler translates the algebraic formula into postfix stack evaluation math.
		// Target Variable = (Finger1X_Coordinate - Box_Left_Offset) * Scale_Multiplier / Effective_Box_Width
		RegisterZone(90, 220, 300, 60, GestureSlide, func() {
			SetLightBrightness("F1.Bathroom.CeilingLight", uint8((Finger1X-90)*100/300))
		})

		// --- CHORDED ANCHOR FINGER RELEASE FALLBACK ---
		// Continuous boundary monitoring check. The exact microsecond the user lets go of
		// Finger 0 (the primary finger holding down the original light button), snap back to standard grid view.
		if Finger0State == 0 {
			UiMode = ModeStandardGrid
		}
	}

	// 5. STATE ROUTING LAYER C: General Event Notification Screen Overlay
	if UiMode == ModeRainingOverlay {
		// Full screen override for urgent sensor alerts (Suddenly-Raining, Washing-Finished)
		DrawSprite(0, 0, "overlays/alert_rain_fullscreen.png")
		DrawText(120, 200, FontMain, ColorWhite, "Suddenly Raining!")
		DrawText(80, 260, FontSmall, ColorWhite, "Tap anywhere to close warning notification")

		// Dismiss overlay immediately on click touch anywhere
		RegisterZone(0, 0, 480, 480, GestureTap, func() {
			UiMode = ModeStandardGrid
		})
	}
}
