package backend

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32Frame   = syscall.NewLazyDLL("user32.dll")
	gdi32Frame    = syscall.NewLazyDLL("gdi32.dll")
	procGetWindowLongW    = user32Frame.NewProc("GetWindowLongW")
	procSetWindowLongW    = user32Frame.NewProc("SetWindowLongW")
	procSetWindowPos      = user32Frame.NewProc("SetWindowPos")
	procGetWindowRect     = user32Frame.NewProc("GetWindowRect")
	procGetClientRect     = user32Frame.NewProc("GetClientRect")
	procSetWindowRgn      = user32Frame.NewProc("SetWindowRgn")
	procCreateRectRgn     = gdi32Frame.NewProc("CreateRectRgn")
	procInvalidateRect    = user32Frame.NewProc("InvalidateRect")
	procMoveWindow        = user32Frame.NewProc("MoveWindow")
)

const (
	WS_CAPTION      = 0x00C00000
	WS_BORDER       = 0x00800000
	WS_DLGFRAME     = 0x00400000
	WS_THICKFRAME   = 0x00040000
	WS_SYSMENU       = 0x00080000
	WS_MINIMIZEBOX   = 0x00020000
	WS_MAXIMIZEBOX   = 0x00010000

	WS_EX_TOOLWINDOW = 0x00000080
	WS_EX_APPWINDOW  = 0x00040000
	WS_EX_WINDOWEDGE = 0x00000100
	WS_EX_CLIENTEDGE = 0x00000200
	WS_EX_STATICEDGE = 0x00020000
	WS_EX_DLGMODALFRAME = 0x00000001

	SWP_FRAMECHANGED = 0x0020
	SWP_NOMOVE       = 0x0002
	SWP_NOSIZE       = 0x0001
	SWP_NOZORDER     = 0x0004
	SWP_NOACTIVATE   = 0x0010
	SWP_SHOWWINDOW   = 0x0040
)

var (
	gwlStyleVal   = uintptr(^uint32(15))
	gwlExStyleVal = uintptr(^uint32(19))
)

type RECT struct {
	left, top, right, bottom int32
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
		winW := wr.right - wr.left
		winH := wr.bottom - wr.top

		style, _, _ := procGetWindowLongW.Call(hwnd, gwlStyleVal)
		removeStyle := uintptr(WS_CAPTION | WS_BORDER | WS_DLGFRAME | WS_THICKFRAME |
			WS_SYSMENU | WS_MINIMIZEBOX | WS_MAXIMIZEBOX)
		procSetWindowLongW.Call(hwnd, gwlStyleVal, style & ^removeStyle)

		exStyle, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyleVal)
		newEx := (exStyle | WS_EX_TOOLWINDOW) & ^uintptr(WS_EX_APPWINDOW | WS_EX_WINDOWEDGE | WS_EX_CLIENTEDGE)
		procSetWindowLongW.Call(hwnd, gwlExStyleVal, newEx)

		procSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(winW), uintptr(winH),
			SWP_FRAMECHANGED|SWP_NOMOVE|SWP_NOZORDER|SWP_NOACTIVATE)

		var cr RECT
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
		cw := cr.right - cr.left
		ch := cr.bottom - cr.top

		hRgn, _, _ := procCreateRectRgn.Call(0, 0, uintptr(cw), uintptr(ch))
		if hRgn != 0 {
			procSetWindowRgn.Call(hwnd, hRgn, 1)
		}
		_ = hRgn

		procShowWindow.Call(hwnd, SW_SHOW)
	}()
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
	w := r.right - r.left
	h := r.bottom - r.top
	procMoveWindow.Call(wm.hwnd, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
}
