package backend

import (
	"syscall"
	"unsafe"
)

var (
	user32Frame    = syscall.NewLazyDLL("user32.dll")
	gdi32Frame     = syscall.NewLazyDLL("gdi32.dll")
	dwmapi         = syscall.NewLazyDLL("dwmapi.dll")
	procGetWindowLongW        = user32Frame.NewProc("GetWindowLongW")
	procSetWindowLongW        = user32Frame.NewProc("SetWindowLongW")
	procSetWindowPos          = user32Frame.NewProc("SetWindowPos")
	procGetClientRect         = user32Frame.NewProc("GetClientRect")
	procSetWindowRgn          = user32Frame.NewProc("SetWindowRgn")
	procCreateRectRgn         = gdi32Frame.NewProc("CreateRectRgn")
	procMoveWindow            = user32Frame.NewProc("MoveWindow")
	procGetWindowRect         = user32Frame.NewProc("GetWindowRect")
	procSetLayeredWindowAttributes = user32Frame.NewProc("SetLayeredWindowAttributes")
	procSetWindowCompositionAttribute = user32Frame.NewProc("SetWindowCompositionAttribute")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	procDwmExtendFrameIntoClientArea = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
)

const (
	WS_CAPTION       = 0x00C00000
	WS_BORDER        = 0x00800000
	WS_DLGFRAME      = 0x00400000
	WS_THICKFRAME    = 0x00040000
	WS_SYSMENU       = 0x00080000
	WS_MINIMIZEBOX   = 0x00020000
	WS_MAXIMIZEBOX   = 0x00010000
	WS_POPUP         = 0x80000000
	WS_VISIBLE       = 0x10000000

	WS_EX_TOOLWINDOW = 0x00000080
	WS_EX_APPWINDOW  = 0x00040000
	WS_EX_LAYERED    = 0x00080000
	WS_EX_TOPMOST    = 0x00000008

	SWP_FRAMECHANGED = 0x0020
	SWP_NOMOVE       = 0x0002
	SWP_NOSIZE       = 0x0001
	SWP_NOZORDER     = 0x0004
	SWP_NOACTIVATE   = 0x0010

	DWMWA_NCRENDERING_POLICY = 2
	DWMNCRP_DISABLED         = 1
	DWMWA_BORDER_COLOR       = 34
	DWMWA_CAPTION_COLOR      = 35
	DWMWA_SYSTEMBACKDROP_TYPE = 38
	DWMSBT_MAINWINDOW        = 1
	DWMSBT_TABBEDWINDOW      = 2
	DWMSBT_ACRYLICBLUR       = 3
)

type MARGINS struct {
	left, right, top, bottom int32
}

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

func ApplyFrameless(hwnd uintptr, width, height int) {
	if hwnd == 0 {
		return
	}

	style, _, _ := procGetWindowLongW.Call(hwnd, ^uintptr(15))

	removeStyle := WS_CAPTION | WS_BORDER | WS_DLGFRAME | WS_THICKFRAME |
		WS_SYSMENU | WS_MINIMIZEBOX | WS_MAXIMIZEBOX
	newStyle := style & ^uintptr(removeStyle)
	newStyle |= WS_POPUP
	procSetWindowLongW.Call(hwnd, ^uintptr(15), newStyle)

	exStyle, _, _ := procGetWindowLongW.Call(hwnd, ^uintptr(19))
	newEx := exStyle | WS_EX_TOOLWINDOW | WS_EX_TOPMOST
	newEx = newEx & ^uintptr(WS_EX_APPWINDOW)
	procSetWindowLongW.Call(hwnd, ^uintptr(19), newEx)

	procSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(width), uintptr(height),
		SWP_FRAMECHANGED|SWP_NOMOVE|SWP_NOZORDER|SWP_NOACTIVATE)

	var cr RECT
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
	cw := cr.right - cr.left
	ch := cr.bottom - cr.top
	hRgn, _, _ := procCreateRectRgn.Call(0, 0, uintptr(cw), uintptr(ch))
	if hRgn != 0 {
		procSetWindowRgn.Call(hwnd, hRgn, 1)
	}

	tryDwmSetWindowAttribute(hwnd, DWMWA_NCRENDERING_POLICY, DWMNCRP_DISABLED, 4)

	margins := MARGINS{left: -1, right: -1, top: -1, bottom: -1}
	procDwmExtendFrameIntoClientArea.Call(hwnd, uintptr(unsafe.Pointer(&margins)))

	tryDwmSetWindowAttribute(hwnd, DWMWA_BORDER_COLOR, 0x00000000, 4)
	tryDwmSetWindowAttribute(hwnd, DWMWA_CAPTION_COLOR, 0x00000000, 4)

	enableAcrylic(hwnd)
	makeLayeredTransparent(hwnd)
}

func tryDwmSetWindowAttribute(hwnd uintptr, attr, value, size uintptr) {
	if procDwmSetWindowAttribute.Find() != nil {
		return
	}
	procDwmSetWindowAttribute.Call(hwnd, attr, uintptr(unsafe.Pointer(&value)), size)
}

func enableAcrylic(hwnd uintptr) {
	if procSetWindowCompositionAttribute.Find() != nil {
		return
	}

	for _, attr := range []int32{19, 20} {
		accent := ACCENTPOLICY{
			AccentState:   4,
			AccentFlags:   0,
			GradientColor: 0x11000000,
			AnimationID:   0,
		}
		data := WINCOMPATTRDATA{
			Attribute: attr,
			Data:      uintptr(unsafe.Pointer(&accent)),
			DataSize:  uint32(unsafe.Sizeof(accent)),
		}
		procSetWindowCompositionAttribute.Call(hwnd, uintptr(unsafe.Pointer(&data)))
	}

	tryDwmSetWindowAttribute(hwnd, DWMWA_SYSTEMBACKDROP_TYPE, DWMSBT_ACRYLICBLUR, 4)
}

func makeLayeredTransparent(hwnd uintptr) {
	exStyle, _, _ := procGetWindowLongW.Call(hwnd, ^uintptr(19))
	procSetWindowLongW.Call(hwnd, ^uintptr(19), exStyle|WS_EX_LAYERED)

	procSetLayeredWindowAttributes.Call(hwnd, 0, 240, 2)
}

type WindowMover struct {
	hwnd uintptr
}

func NewWindowMover(hwnd uintptr) *WindowMover {
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
