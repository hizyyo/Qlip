package backend

import (
	"syscall"
)

var (
	user32Toggle       = syscall.NewLazyDLL("user32.dll")
	procShowWindow     = user32Toggle.NewProc("ShowWindow")
	procSetForegroundWindow = user32Toggle.NewProc("SetForegroundWindow")
	procIsWindowVisible = user32Toggle.NewProc("IsWindowVisible")
)

const (
	SW_HIDE    = 0
	SW_SHOW    = 5
	SW_RESTORE = 9
)

type Toggle struct {
	hwnd uintptr
}

func NewToggle(hwnd uintptr) *Toggle {
	return &Toggle{hwnd: hwnd}
}

func (t *Toggle) Show() {
	if t.hwnd == 0 {
		return
	}
	procShowWindow.Call(t.hwnd, SW_RESTORE)
	procSetForegroundWindow.Call(t.hwnd)
}

func (t *Toggle) Hide() {
	if t.hwnd == 0 {
		return
	}
	procShowWindow.Call(t.hwnd, SW_HIDE)
}

func (t *Toggle) IsVisible() bool {
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
