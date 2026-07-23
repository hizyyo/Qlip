package backend

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procGlobalLock       = kernel32.NewProc("GlobalLock")
	procGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	procGlobalSize       = kernel32.NewProc("GlobalSize")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procSetClipboardData          = user32.NewProc("SetClipboardData")
	procEmptyClipboard            = user32.NewProc("EmptyClipboard")
	procGlobalAlloc               = kernel32.NewProc("GlobalAlloc")
)

const (
	CF_TEXT      = 1
	CF_UNICODETEXT = 13
	CF_DIB       = 8
	GMEM_MOVABLE = 2
)

type ClipboardChange struct {
	Type    string
	Text    string
	ImageID string
}

type Clipboard struct {
	lastContent string
	onChange    func(ClipboardChange)
}

func NewClipboard(onChange func(ClipboardChange)) *Clipboard {
	return &Clipboard{onChange: onChange}
}

func (c *Clipboard) Start() {
	go c.poll()
}

func (c *Clipboard) poll() {
	time.Sleep(1 * time.Second)
	for {
		change := readClipboard()
		if change != nil && change.Text != c.lastContent {
			c.lastContent = change.Text
			c.onChange(*change)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func readClipboard() *ClipboardChange {
	procOpenClipboard.Call(0)
	defer procCloseClipboard.Call()

	textAvail, _, _ := procIsClipboardFormatAvailable.Call(CF_UNICODETEXT)
	if textAvail != 0 {
		text := readTextInner()
		if text != "" {
			return &ClipboardChange{Type: "text", Text: text}
		}
	}

	imgAvail, _, _ := procIsClipboardFormatAvailable.Call(CF_DIB)
	if imgAvail != 0 {
		return &ClipboardChange{Type: "image", ImageID: "dib"}
	}

	return nil
}

func readTextInner() string {
	handle, _, _ := procGetClipboardData.Call(CF_UNICODETEXT)
	if handle == 0 {
		return ""
	}

	ptr, _, _ := procGlobalLock.Call(handle)
	if ptr == 0 {
		return ""
	}
	defer procGlobalUnlock.Call(handle)

	size, _, _ := procGlobalSize.Call(handle)
	if size == 0 {
		return ""
	}

	buf := make([]uint16, size/2)
	copy(buf, unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(buf)))

	return syscall.UTF16ToString(buf)
}

func WriteClipboardText(text string) error {
	procOpenClipboard.Call(0)
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	utf16 := syscall.StringToUTF16(text)
	size := len(utf16) * 2

	handle, _, _ := procGlobalAlloc.Call(GMEM_MOVABLE, uintptr(size))
	if handle == 0 {
		return syscall.GetLastError()
	}

	ptr, _, _ := procGlobalLock.Call(handle)
	if ptr == 0 {
		return syscall.GetLastError()
	}

	copy(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), size),
		unsafe.Slice((*byte)(unsafe.Pointer(&utf16[0])), size))

	procGlobalUnlock.Call(handle)
	procSetClipboardData.Call(CF_UNICODETEXT, handle)
	return nil
}
