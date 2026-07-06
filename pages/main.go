package pages

import (
	. "github.com/jurgen-kluft/go-esp32-ui/ui"
)

// Depending on a variable like PageID, we can route to different page functions.
// This allows us to have a single entry point for the page rendering logic, while
// still being able to render different pages based on the current state of the application.
func PageMain() {
	if VarEq(UIPage, PageOverview) {
		PageOverviewFn()
	} else if VarEq(UIPage, Page1stFloorBathroom) {
		Page1stFloorBathroomFn()
	} else {
		// Handle unknown page ID or default behavior
	}
}
