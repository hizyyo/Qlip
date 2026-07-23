package backend

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32Toggle       = syscall.NewLazyDLL("user32.dll")
	procShowWindow     = user32Toggle.NewProc("ShowWindow")
	procSetForegroundWindow = user32Toggle.NewProc("SetForegroundWindow")
	procIsWindowVisible = user32Toggle.NewProc("IsWindowVisible")
	procBringWindowToTop = user32Toggle.NewProc("BringWindowToTop")
	procGetSystemMetrics = user32Toggle.NewProc("GetSystemMetrics")
)

const (
	SW_HIDE    = 0
	SW_SHOW    = 5
	SW_RESTORE = 9

	HWND_TOPMOST = ^uintptr(0)
	SWP_SHOWWINDOW = 0x0040

	animSteps  = 20
	animDelay  = 10 * time.Millisecond
	pillW = 64
	pillH = 36
	marginRight = 16
	marginTop   = 16
)

type Toggle struct {
	hwnd   uintptr
	fullW  int
	fullH  int
	fullX  int
	fullY  int
}

func NewToggle(hwnd uintptr) *Toggle {
	x, y, w, h := getWindowRect(hwnd)
	return &Toggle{
		hwnd:  hwnd,
		fullW: w,
		fullH: h,
		fullX: x,
		fullY: y,
	}
}

func (t *Toggle) Show() {
	if t.hwnd == 0 {
		return
	}
	procShowWindow.Call(t.hwnd, SW_RESTORE)
	procSetWindowPos.Call(t.hwnd, HWND_TOPMOST,
		uintptr(t.fullX), uintptr(t.fullY),
		uintptr(t.fullW), uintptr(t.fullH),
		SWP_SHOWWINDOW)
	procSetForegroundWindow.Call(t.hwnd)
	procBringWindowToTop.Call(t.hwnd)
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

func (t *Toggle) AnimatedToggle() {
	if t.IsVisible() {
		t.animatedHide()
	} else {
		t.animatedShow()
	}
}

func (t *Toggle) animatedHide() {
	if t.hwnd == 0 {
		return
	}

	x, y, w, h := getWindowRect(t.hwnd)
	sw, _ := getScreenSize()
	targetX := sw - pillW - marginRight
	targetY := marginTop

	for i := 1; i <= animSteps; i++ {
		p := float64(i) / animSteps
		e := easeInOutCubic(p)
		cx := lerp(x, targetX, e)
		cy := lerp(y, targetY, e)
		cw := lerp(w, pillW, e)
		ch := lerp(h, pillH, e)
		procSetWindowPos.Call(t.hwnd, 0,
			uintptr(cx), uintptr(cy),
			uintptr(cw), uintptr(ch),
			SWP_NOZORDER|SWP_NOACTIVATE)
		time.Sleep(animDelay)
	}

	procShowWindow.Call(t.hwnd, SW_HIDE)
	procSetWindowPos.Call(t.hwnd, HWND_TOPMOST,
		uintptr(t.fullX), uintptr(t.fullY),
		uintptr(t.fullW), uintptr(t.fullH),
		SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
}

func (t *Toggle) animatedShow() {
	if t.hwnd == 0 {
		return
	}

	sw, _ := getScreenSize()
	startX := sw - pillW - marginRight
	startY := marginTop

	procShowWindow.Call(t.hwnd, SW_SHOW)
	procSetWindowPos.Call(t.hwnd, HWND_TOPMOST,
		uintptr(startX), uintptr(startY),
		uintptr(pillW), uintptr(pillH),
		SWP_NOACTIVATE)

	time.Sleep(30 * time.Millisecond)

	for i := 1; i <= animSteps; i++ {
		p := float64(i) / animSteps
		e := easeInOutCubic(p)
		cx := lerp(startX, t.fullX, e)
		cy := lerp(startY, t.fullY, e)
		cw := lerp(pillW, t.fullW, e)
		ch := lerp(pillH, t.fullH, e)
		procSetWindowPos.Call(t.hwnd, 0,
			uintptr(cx), uintptr(cy),
			uintptr(cw), uintptr(ch),
			SWP_NOZORDER|SWP_NOACTIVATE)
		time.Sleep(animDelay)
	}

	procSetWindowPos.Call(t.hwnd, HWND_TOPMOST,
		uintptr(t.fullX), uintptr(t.fullY),
		uintptr(t.fullW), uintptr(t.fullH),
		SWP_NOACTIVATE)
	procSetForegroundWindow.Call(t.hwnd)
	procBringWindowToTop.Call(t.hwnd)
}

func getWindowRect(hwnd uintptr) (x, y, w, h int) {
	var r struct{ left, top, right, bottom int32 }
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return int(r.left), int(r.top), int(r.right - r.left), int(r.bottom - r.top)
}

func getScreenSize() (int, int) {
	sw, _, _ := procGetSystemMetrics.Call(0)
	sh, _, _ := procGetSystemMetrics.Call(1)
	return int(sw), int(sh)
}

func lerp(a, b int, t float64) int {
	return int(float64(a) + float64(b-a)*t)
}

func easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - pow(2-2*t, 3)/2
}

func pow(x float64, n int) float64 {
	r := 1.0
	for i := 0; i < n; i++ {
		r *= x
	}
	return r
}
