package backend

import (
	"syscall"
	"unsafe"
)

var (
	user32Toggle = syscall.NewLazyDLL("user32.dll")
	procFindWindowW       = user32Toggle.NewProc("FindWindowW")
	procShowWindow        = user32Toggle.NewProc("ShowWindow")
	procSetForegroundWindow = user32Toggle.NewProc("SetForegroundWindow")
	procIsWindowVisible   = user32Toggle.NewProc("IsWindowVisible")
	procGetWindowTextW    = user32Toggle.NewProc("GetWindowTextW")
	procEnumWindows       = user32Toggle.NewProc("EnumWindows")
)

const (
	SW_HIDE = 0
	SW_SHOW = 5
	SW_RESTORE = 9
)

type Toggle struct {
	hwnd uintptr
}

func NewToggle(title string) *Toggle {
	return &Toggle{hwnd: findWindowByTitle(title)}
}

func findWindowByTitle(title string) uintptr {
	utf16 := syscall.StringToUTF16(title)
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(&utf16[0])))
	return hwnd
}

func (t *Toggle) Show() {
	if t.hwnd == 0 {
		t.hwnd = findWindowByTitle("ClipFlow")
	}
	if t.hwnd != 0 {
		procShowWindow.Call(t.hwnd, SW_RESTORE)
		procSetForegroundWindow.Call(t.hwnd)
	}
}

func (t *Toggle) Hide() {
	if t.hwnd == 0 {
		t.hwnd = findWindowByTitle("ClipFlow")
	}
	if t.hwnd != 0 {
		procShowWindow.Call(t.hwnd, SW_HIDE)
	}
}

func (t *Toggle) IsVisible() bool {
	if t.hwnd == 0 {
		t.hwnd = findWindowByTitle("ClipFlow")
	}
	if t.hwnd == 0 {
		return false
	}
	ret, _, _ := procIsWindowVisible.Call(t.hwnd)
	return ret != 0
}

func (t *Toggle) Toggle() {
	if t.IsVisible() {
		t.Hide()
	} else {
		t.Show()
	}
}

func (t *Toggle) RefreshHWND() {
	t.hwnd = findWindowByTitle("ClipFlow")
}
