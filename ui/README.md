# ESP32 UI Framework

[ FRAME START ]
       │
       ▼
 1. Update Timers ──────► [ Subtract Δt from all active timer slots ]
       │
       ▼
 2. Update Inputs ─────► [ Inject fresh multi-touch / mouse X, Y coordinates ]
       │
       ▼
 3. Scan Gestures ─────► [ Match touch coordinates against last frame's hitboxes ]
       │                     └─► If match: Jump VM Program Counter to Subroutine Address
       ▼
 4. Execute Bytecode ──► [ Clear Active Hitbox List, Reset Skip Stacks ]
       │                     └─► Interpret Main Render Bytecode from PC = 0 to OpExitFrame
       ▼
[ FRAME END ]  ──────► [ Trigger DMA Transfer: Flush Frame Buffer to ST7701 Display ]
