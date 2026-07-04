package ui

import (
	"fmt"
	"runtime"

	"github.com/jurgen-kluft/go-gui-app/backend"
	metalbackend "github.com/jurgen-kluft/go-gui-app/backend/metal"
	"github.com/jurgen-kluft/go-gui-app/examples/common"
	"github.com/jurgen-kluft/go-gui-app/imgui"
	"github.com/jurgen-kluft/go-gui-app/imguizmo"
)

var currentBackend backend.Backend[metalbackend.MetalWindowFlags]

func init() {
	runtime.LockOSThread()
}

func begin() {
	//Initialize()

	currentBackend, _ = backend.CreateBackend(metalbackend.NewMetalBackend())
	currentBackend.SetAfterCreateContextHook(common.AfterCreateContext)
	currentBackend.SetBeforeDestroyContextHook(common.BeforeDestroyContext)
	currentBackend.SetTargetFPS(120) // enable ProMotion

	currentBackend.SetBgColor(imgui.NewVec4(0.45, 0.55, 0.6, 1.0))

	currentBackend.CreateWindow("Hello from cimgui-go", 1200, 900)

	currentBackend.SetDropCallback(func(p []string) {
		fmt.Printf("drop triggered: %v", p)
	})

	currentBackend.SetCloseCallback(func() {
		fmt.Println("window is closing")
	})

	currentBackend.Run(Loop)
}

func Loop() {
	imgui.ClearSizeCallbackPool()
	imguizmo.BeginFrame()
}
