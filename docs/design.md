# UI

We develop the UI in Golang using constrained functionality of Go and only a given set of external functions like DrawText, DrawSprite, DrawVar, StartTimer, etc.

The UI code should mainly use logic and control flow, but to get a better grip of what is not supported, the following list of unsupported Golang functionality should give a good overview of what is not supported:

* no arrays, no slices, no maps, no structs, no pointers
* no allocation of memory (no new, no make, no append)
* no reflection
* no goroutines, no channels
* no defer, no panic, no recover
* no interfaces, no type assertions
* no closures, no anonymous functions
* no package imports, no external packages

The UI code can be compiled to a bytecode format that is interpreted by a custom virtual-machine running on the ESP32. The virtual machine is implemented in C like C++.

This means that the UI code is a VM binary and we can update the UI without having to recompile the entire firmware. The UI can be updated over-the-air (OTA) or on an SD card.

## State

The full state consists of predefined variables where each variable has an ID.
Through this we can update the state externally and the UI will automatically reflect the new state.

## ID

An ID is a 32-bit integer that is used to identify a variable or asset in the UI code. The ID should stay stable, that is why they need to be predefined, so even if variables or assets are added or removed, the UI code can be updated without having to recompile the entire firmware.

[Type:8, Index:24] = 32 bits

Types:

* TypeVariableU8        = 1
* TypeVariableU16       = 2
* TypeVariableU32       = 3
* TypeVariableS8        = 4
* TypeVariableS16       = 5
* TypeVariableS32       = 6
* TypeVariableF32       = 7
* TypeAssetColorPalette = 128
* TypeAssetImage        = 129
* TypeAssetFont         = 130

## Global Variable Examples

Global variables are predefined and can be used in the UI code. Each variable has a type and an index. The type is used to determine how to interpret the variable's value, and the index is used to identify the variable in the state.

e.g. 
"GroundFloor.Bathroom.CeilingLight.OnOff"            = TypeVariableU8, Index = 0
"GroundFloor.Bathroom.CeilingLight.Brightness"       = TypeVariableU8, Index = 1
"GroundFloor.Bathroom.CeilingLight.Color"            = TypeVariableU32, Index = 0

"Sensor.Inside.Temp"                                 = TypeVariableF32, Index = 1
"Sensor.Inside.Humidity"                             = TypeVariableF32, Index = 2

"Sensor.Weather.Temp"                                = TypeVariableF32, Index = 3
"Sensor.Weather.Humidity"                            = TypeVariableF32, Index = 4
"Sensor.Weather.Rain"                                = TypeVariableF32, Index = 5

"System.Year"                                        = TypeVariableU16, Index = 0
"System.Month"                                       = TypeVariableU8, Index = 2
"System.Day"                                         = TypeVariableU8, Index = 3
"System.WeekDay"                                     = TypeVariableU8, Index = 4
"System.Hour"                                        = TypeVariableU8, Index = 5
"System.Minute"                                      = TypeVariableU8, Index = 6
"System.Second"                                      = TypeVariableU8, Index = 7

## Local Variables

The UI code can define local variables that are only visible within the block they are defined in. Local variables are stored on the stack and are automatically cleaned up when the block is exited. Local variables can be used to store temporary values that are only needed within the block.

Just like the global variables, local variables have a type and an index. The type is used to determine how to interpret the variable's value, and the index is used to identify the variable in the stack frame.

## System Calls

System calls have to be wrapped in a code block and are called like a function. The system call is compiled into a binary format that can be interpreted by the VM. The system call has an ID that is used to identify the system call in the VM.

## Assets Examples

"fonts/Roboto-Regular-20.ttf" = TypeAssetFont, Index = 0
"fonts/Roboto-Regular-30.ttf" = TypeAssetFont, Index = 1
"fonts/Roboto-Regular-40.ttf" = TypeAssetFont, Index = 2

"palettes/palette-house.pal" = TypeAssetColorPalette, Index = 0

"images/house.png" = TypeAssetImage, Index = 0
"images/house2.png" = TypeAssetImage, Index = 1
"images/light-button-on.png" = TypeAssetImage, Index = 2
"images/light-button-off.png" = TypeAssetImage, Index = 3

## Code -> AST -> VM Binary 

The Golang UI code is parsed into an Abstract Syntax Tree (AST) and then compiled into a binary format that can be interpreted by the VM  running on the ESP32. Golang supports parsing and compiling code into an AST, which we can then use to generate the custom binary format for the VM.

A set of functions are provided as system calls to the VM. These functions can be used in the UI code to interact with the state and update the UI. 

We treat code as 'blocks', for example the code within an if condition is treated as a 'block' as well as the true and false branches of the if condition. Each block is compiled into a binary format that can be interpreted by the VM. So the 'if' opcode has an offset that points to the condition block, the true block and the false block. The VM will execute the condition block and then jump to the true or false block based on the result of the condition. Each block has a return opcode that tells the VM to return to the previous block, so the VM also keeps track of a call stack and the current block being executed. A code block is able to 'return' values to the calling block, so the calling block can use the returned values in its own code, this is mainly used for the if condition where the condition block returns a boolean value to the calling block.

## UI VM

The UI VM is implemented in C and runs on the ESP32. The VM interprets the binary format generated from the UI code and executes the code. The VM has a set of predefined functions that can be used to interact with the state and update/draw the UI. 

Upon boot, the VM executes the following 'functions':

* `Initialize()`

Then the VM executes the following 'functions' every tick:

* `UpdateTimers()` - Active timers are updated
* `UpdateInput()`  - Input is read
* `UpdatePage()`   - The current page OnTick() function is called
* `UpdateEvents()` - Input is processed using the registered interaction zones

## Menu Code, Available Functions

* `DrawBackground(imageId)` - Draws the background image with the given ID
* `DrawSprite(imageId, x, y)` - Draws the sprite with the given ID at the given position
* `DrawText(fontId, text, x, y, color)` - Draws the text with the given font ID at the given position
* `DrawVar(fontId, varId, x, y, color)` - Draws the variable with the given ID at the given position
* `StartTimer(timerId, duration)` - Starts a timer with the given ID and duration in milliseconds.
* `StopTimer(timerId)` - Stops the timer with the given ID.
* `GetTimer(timerId)` - Returns the remaining time of the timer with the given ID in milliseconds.
* `SetLightOnOff(lightId, onOff)` - Sets the light with the given ID to on or off.
* `IsLightOn(lightId)` - Returns the on/off state of the light with the given ID
* `SetLightBrightness(lightId, brightness)` - Sets the brightness of the light with the given ID
* `GetLightBrightness(lightId)` - Returns the brightness of the light with the given ID
* `SetLightColor(lightId, color)` - Sets the color of the light with the given ID
* `GetLightColor(lightId)` - Returns the color of the light with the given ID

## UI Page

A UI page executes the following 'functions':

* `OnUpdate()` - This function is called every update

## Interaction Zones

The code can register interaction zones with the VM. An interaction zone is a rectangular area on the screen that can be interacted with by the user. The interaction zone can be registered with a unique ID, event filter and a callback function that will be called when the user interacts with the zone. The callback function is basically a code block that will be executed when the user interacts with the zone. The event filter is used to filter the events that will trigger the callback function, for example we can filter for 'tap' events or 'swipe' events. The interaction zone can also be registered with a unique ID, so we can update the interaction zone's properties (position, size, etc.) at runtime.

## Color Palettes

Color palettes are sent to the client and registered with a asset ID. The ID is used to reference the color palette in the sprite data. 

## Sprites

Sprites are small images that can be used in the UI, they are send to the client and registered with a asset ID. The ID is used to reference the sprite in the UI code.

## Fonts

Fonts are also sent to the client and registered with a asset ID. The ID is used to reference the font in the UI code.
