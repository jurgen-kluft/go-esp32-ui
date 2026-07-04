package pages

import (
	. "github.com/jurgen-kluft/go-esp32-ui/ui"
)

type LightStatus struct {
	OnOff      int8
	Brightness int8
	Warm       int8
	Color      int32
}

type UiState struct {
	UiPage int

	// Environmental Sensor Data
	InsideTemp  float32
	InsideHum   float32
	OutsideTemp float32
	OutsideHum  float32

	// Date
	Year  uint16
	Month uint8
	Day   uint8

	// Time
	Hour   uint8
	Minute uint8
	Second uint8

	// Basement
	BasementFrontRoomLight1 LightStatus
	BasementFrontRoomLight2 LightStatus
	BasementBackRoomLight1  LightStatus

	// Ground Floor
	GroundFloorEntranceLight    LightStatus
	GroundFloorBathroomLight    LightStatus
	GroundFloorKitchenLight1    LightStatus
	GroundFloorKitchenLight2    LightStatus
	GroundFloorDiningRoomLight1 LightStatus
	GroundFloorLivingRoomLight1 LightStatus
	GroundFloorLivingRoomLight2 LightStatus
	GroundFloorBedRoomLight1    LightStatus
	GroundFloorBedRoomLight2    LightStatus
	GroundFloorStairWayLight1   LightStatus

	// First Floor
	FirstFloorBathRoomLight1    LightStatus
	FirstFloorBathRoomLight2    LightStatus
	FirstFloorStudyRoomLight1   LightStatus
	FirstFloorStudyRoomLight2   LightStatus
	FirstFloorLivingRoomLight1  LightStatus
	FirstFloorLivingRoomLight2  LightStatus
	FirstFloorLivingRoomLight3  LightStatus
	FirstFloorLivingRoomLight4  LightStatus
	FirstFloorWashingRoomLight1 LightStatus
	FirstFloorWashingRoomLight2 LightStatus
	FirstFloorStairWayLight1    LightStatus

	// Second Floor
	SecondFloorBathRoomLight1      LightStatus
	SecondFloorBathRoomLight2      LightStatus
	SecondFloorBedRoomLight1       LightStatus
	SecondFloorBedRoomLight2       LightStatus
	SecondFloorMasterBedRoomLight1 LightStatus
	SecondFloorMasterBedRoomLight2 LightStatus
	SecondFloorMasterBedRoomLight3 LightStatus
	SecondFloorMasterBedRoomLight4 LightStatus
	SecondFloorStairWayLight1      LightStatus
}
