package pages

import (
	. "github.com/jurgen-kluft/go-esp32-ui/ui"
)

func Page1stFloorBathroomFn() {
	// 1. Establish a clean frame baseline by flooding the 480x480 canvas via hardware
	ClearScreen(ColorCharcoal)

	// 2. Global Top Border & Navigation Banner
	DrawSprite(0, 0, BgCharcoalTopBanner)
	DrawText(FontMain, "Basement - Bathroom", 20, 15, ColorWhite)

	// 3. STATE ROUTING LAYER A: Standard Operational Grid View
	if VarEq(UIMode, ModeStandardGrid) {

		// Define UI Grid positioning constraints natively in Go
		const startX = 40
		const startY = 100
		const btnSize = 120
		const spacing = 140

		// Small date on the left top corner of the grid
		DrawVar(FontSmall, DateString, 20, 60, ColorWhite)

		// Small time on the right top corner of the grid
		DrawVar(FontSmall, TimeString, 400, 60, ColorWhite)

		// Single-Tap: Instantly toggles the light status variable locally on the ESP32
		if RegisterZone(startX, startY, btnSize, btnSize, GestureSingleTap) {
			SetLightToggle(GroundfloorBathroomCeilingLight_State)
		}

		// Grid Column 0, Row 0: Main Ceiling Light Button Container
		if IsLightOn(GroundfloorBathroomCeilingLight_State) {
			DrawSprite(startX, startY, BtnGoldOn120x120)
			DrawText(FontSmall, "Ceiling (ON)", startX+15, startY+80, ColorWhite)
		} else {
			DrawSprite(startX, startY, BtnGreyOff120x120)
			DrawText(FontSmall, "Ceiling (OFF)", startX+15, startY+80, ColorWhite)
		}

		// Grid Column 1, Row 0: Mirror Light Button Container
		if RegisterZone(startX+spacing, startY, btnSize, btnSize, GestureSingleTap) {
			SetLightToggle(GroundfloorBathroomMirrorLight_State)
		}

		if IsLightOn(GroundfloorBathroomMirrorLight_State) {
			DrawSprite(startX+spacing, startY, BtnGoldOn120x120)
			DrawText(FontSmall, "Mirror (ON)", startX+spacing+20, startY+80, ColorWhite)
		} else {
			DrawSprite(startX+spacing, startY, BtnGreyOff120x120)
			DrawText(FontSmall, "Mirror (OFF)", startX+spacing+20, startY+80, ColorWhite)
		}

		// Press & Hold: Hides the main grid and enters the advanced multi-touch dimmer overlay
		if RegisterZone(startX, startY, btnSize, btnSize, GestureSingleHold|GestureFinger0) {
			VarAssign(&UIMode, ModeDimmerOverlay)
		}

		// --- DOUBLE TAP SCREEN ESCAPE ACCELERATOR ---
		// A double-tap anywhere on the main body bounds returns the user to the Floor Overview layout
		if RegisterZone(0, 50, 480, 430, GestureDoubleTap) {
			VarAssign(&UIMode, ModeFloorOverview)
		}
	}

	// 4. STATE ROUTING LAYER B: Advanced Multi-Touch Dimmer Overlay View
	if VarEq(UIMode, ModeDimmerOverlay) {
		// Blit a pre-rendered translucent black mask to fade back the underlying screen elements safely
		DrawSprite(0, 0, OverlayDimBackdrop)

		// Render the modal overlay popup dialogue block frame
		DrawSprite(60, 100, OverlaySliderCard360x280)
		DrawText(FontMain, "Ceiling Brightness", 90, 130, ColorBlack)

		// Display current slider value via text tracking variable changes dynamically
		// (In a future step, our text compile pipeline will evaluate this variable)
		//DrawText(90, 170, FontSmall, ColorDarkGrey, fmt.Sprintf("Intensity: %d%%", GetLightBrightness("F1.Bathroom.CeilingLight")))
		DrawText(FontSmall, "Intensity:", 90, 170, ColorDarkGrey)
		DrawVar(FontSmall, GroundfloorBathroomCeilingLight_Brightness, 200, 170, ColorDarkGrey)

		// --- ADVANCED SLIDER DRAG GESTURE ---
		// We track the slider bar boundaries. While Finger 1 slides horizontally inside it,
		// the AST compiler translates the algebraic formula into postfix stack evaluation math.
		// Target Variable = (Finger1X_Coordinate - Box_Left_Offset) * Scale_Multiplier / Effective_Box_Width
		if RegisterZone(90, 220, 300, 60, GestureSlide|GestureFinger1) {
			SetLightBrightness(GroundfloorBathroomCeilingLight_Brightness, uint32((VarToInt32(Finger1X)-90)*100/300))
		}

		// --- CHORDED ANCHOR FINGER RELEASE FALLBACK ---
		// Continuous boundary monitoring check. The exact microsecond the user lets go of
		// Finger 0 (the primary finger holding down the original light button), snap back to standard grid view.
		if VarEq(Finger0State, 0) {
			VarAssign(&UIMode, ModeStandardGrid)
		}
	}

	// 5. STATE ROUTING LAYER C: General Event Notification Screen Overlay
	if VarEq(UIMode, ModeRainingOverlay) {
		// Full screen override for urgent sensor alerts (Suddenly-Raining, Washing-Finished)
		DrawSprite(0, 0, OverlayRainFullscreen)
		DrawText(FontMain, "Suddenly Raining!", 120, 200, ColorWhite)
		DrawText(FontSmall, "Tap anywhere to close warning notification", 80, 260, ColorWhite)

		// Dismiss overlay immediately on click touch anywhere
		if RegisterZone(0, 0, 480, 480, GestureSingleTap) {
			VarAssign(&UIMode, ModeStandardGrid)
		}

		// Check if the RainingOverlayTimer has completed its countdown
		if IsTimerDone(RainingOverlayTimerId) {
			VarAssign(&UIMode, ModeStandardGrid)
		}
	}
}
