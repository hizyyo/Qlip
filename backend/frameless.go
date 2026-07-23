package backend

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32Frame   = syscall.NewLazyDLL("user32.dll")
	gdi32Frame    = syscall.NewLazyDLL("gdi32.dll")
	dwmapiFrame   = syscall.NewLazyDLL("dwmapi.dll")
	procGetWindowLongW    = user32Frame.NewProc("GetWindowLongW")
	procSetWindowLongW    = user32Frame.NewProc("SetWindowLongW")
	procSetWindowPos      = user32Frame.NewProc("SetWindowPos")
	procGetWindowRect     = user32Frame.NewProc("GetWindowRect")
	procGetClientRect     = user32Frame.NewProc("GetClientRect")
	procSetWindowRgn      = user32Frame.NewProc("SetWindowRgn")
	procCreateRectRgn     = gdi32Frame.NewProc("CreateRectRgn")
	procMoveWindow        = user32Frame.NewProc("MoveWindow")
	procDwmSetWindowAttribute = dwmapiFrame.NewProc("DwmSetWindowAttribute")
	procSetWindowCompositionAttribute = user32Frame.NewProc("SetWindowCompositionAttribute")
	procDwmExtendFrameIntoClientArea  = dwmapiFrame.NewProc("DwmExtendFrameIntoClientArea")
)

const (
	WS_CAPTION      = 0x00C00000
	WS_BORDER       = 0x00800000
	WS_DLGFRAME     = 0x00400000
	WS_THICKFRAME   = 0x00040000
	WS_SYSMENU       = 0x00080000
	WS_MINIMIZEBOX   = 0x00020000
	WS_MAXIMIZEBOX   = 0x00010000

	WS_EX_TOOLWINDOW   = 0x00000080
	WS_EX_APPWINDOW    = 0x00040000
	WS_EX_WINDOWEDGE   = 0x00000100
	WS_EX_CLIENTEDGE    = 0x00000200
	WS_EX_STATICEDGE    = 0x00020000
	WS_EX_LAYERED       = 0x00080000
	WS_EX_TRANSPARENT   = 0x00000020

	SWP_FRAMECHANGED = 0x0020
	SWP_NOMOVE       = 0x0002
	SWP_NOSIZE       = 0x0001
	SWP_NOZORDER     = 0x0004
	SWP_NOACTIVATE   = 0x0010
)

var (
	gwlStyleVal   = uintptr(^uint32(15))
	gwlExStyleVal = uintptr(^uint32(19))

	DWMWA_SYSTEMBACKDROP_TYPE = uintptr(38)
	DWMSBT_ACRYLIC            = uintptr(4)
	DWMSBT_MAINWINDOW         = uintptr(2)
)

type RECT struct {
	left, top, right, bottom int32
}

type ACCENTPOLICY struct {
	AccentState   uint32
	AccentFlags   uint32
	GradientColor uint32
	AnimationID   uint32
}

type WINCOMPATTRDATA struct {
	Attribute int32
	Data      uintptr
	DataSize  uint32
}

func SetFramelessOverlay(title string) {
	go func() {
		time.Sleep(200 * time.Millisecond)

		utf16 := syscall.StringToUTF16(title)
		hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(&utf16[0])))
		if hwnd == 0 {
			return
		}

		var wr RECT
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
		winW := uintptr(wr.right - wr.left)
		winH := uintptr(wr.bottom - wr.top)

		style, _, _ := procGetWindowLongW.Call(hwnd, gwlStyleVal)
		removeStyle := uintptr(WS_CAPTION | WS_BORDER | WS_DLGFRAME | WS_THICKFRAME |
			WS_SYSMENU | WS_MINIMIZEBOX | WS_MAXIMIZEBOX)
		procSetWindowLongW.Call(hwnd, gwlStyleVal, style & ^removeStyle)

		exStyle, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyleVal)
		newEx := exStyle | WS_EX_TOOLWINDOW | WS_EX_LAYERED
		newEx = newEx & ^uintptr(WS_EX_APPWINDOW | WS_EX_WINDOWEDGE | WS_EX_CLIENTEDGE)
		procSetWindowLongW.Call(hwnd, gwlExStyleVal, newEx)

		procSetWindowPos.Call(hwnd, 0, 0, 0, winW, winH,
			SWP_FRAMECHANGED|SWP_NOMOVE|SWP_NOZORDER|SWP_NOACTIVATE)

		var cr RECT
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
		cw := uintptr(cr.right - cr.left)
		ch := uintptr(cr.bottom - cr.top)
		hRgn, _, _ := procCreateRectRgn.Call(0, 0, cw, ch)
		if hRgn != 0 {
			procSetWindowRgn.Call(hwnd, hRgn, 1)
		}

		enableAcrylicDWM(hwnd)

		procShowWindow.Call(hwnd, SW_SHOW)
	}()
}

func enableAcrylicDWM(hwnd uintptr) {
	var attr uint32 = 4
	if procDwmSetWindowAttribute.Find() == nil {
		ret, _, _ := procDwmSetWindowAttribute.Call(
			hwnd, 38,
			uintptr(unsafe.Pointer(&attr)),
			4,
		)
		if ret == 0 {
			return
		}
	}

	if procSetWindowCompositionAttribute.Find() == nil {
		accent := ACCENTPOLICY{
			AccentState:   4,
			AccentFlags:   0,
			GradientColor: 0x19000000,
			AnimationID:   0,
		}
		data := WINCOMPATTRDATA{
			Attribute:    19,
			Data:         uintptr(unsafe.Pointer(&accent)),
			DataSize:     uint32(unsafe.Sizeof(accent)),
		}
		ret, _, _ := procSetWindowCompositionAttribute.Call(hwnd, uintptr(unsafe.Pointer(&data)))
		if ret != 0 {
			return
		}
		accent.GradientColor = 0x1A000000
		data.Attribute = 20
		ret, _, _ = procSetWindowCompositionAttribute.Call(hwnd, uintptr(unsafe.Pointer(&data)))
		if ret != 0 {
			return
		}
	}

	if procDwmExtendFrameIntoClientArea.Find() == nil {
		margins := [4]int32{-1, -1, -1, -1}
		procDwmExtendFrameIntoClientArea.Call(hwnd, uintptr(unsafe.Pointer(&margins[0])))
	}
}

type WindowMover struct {
	hwnd uintptr
}

func NewWindowMover(title string) *WindowMover {
	utf16 := syscall.StringToUTF16(title)
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(&utf16[0])))
	return &WindowMover{hwnd: hwnd}
}

func (wm *WindowMover) Move(x, y int) {
	if wm.hwnd == 0 {
		return
	}
	var r RECT
	procGetWindowRect.Call(wm.hwnd, uintptr(unsafe.Pointer(&r)))
	w := uintptr(r.right - r.left)
	h := uintptr(r.bottom - r.top)
	procMoveWindow.Call(wm.hwnd, uintptr(x), uintptr(y), w, h, 1)
}
