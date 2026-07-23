package backend

import (
	"runtime"
	"syscall"
	"time"
)

var (
	hotkeyDll        = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = hotkeyDll.NewProc("GetAsyncKeyState")
)

const (
	MOD_ALT = 0x0001
)

type Hotkey struct {
	onPress func()
	stopCh  chan struct{}
}

func NewHotkey(onPress func()) *Hotkey {
	return &Hotkey{
		onPress: onPress,
		stopCh:  make(chan struct{}),
	}
}

func (h *Hotkey) Register(mod, vk int) error {
	go h.poll(mod, vk)
	return nil
}

func (h *Hotkey) poll(mod, vk int) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var pressed bool
	for {
		select {
		case <-h.stopCh:
			return
		default:
		}

		modDown := false
		if mod == MOD_ALT {
			state, _, _ := procGetAsyncKeyState.Call(0x12)
			modDown = state&0x8000 != 0
		}

		vkDown := false
		state, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
		vkDown = state&0x8000 != 0

		if modDown && vkDown && !pressed {
			pressed = true
			h.onPress()
		} else if !modDown || !vkDown {
			pressed = false
		}

		time.Sleep(100 * time.Millisecond)
	}
}
