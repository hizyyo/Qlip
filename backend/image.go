package backend

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)



type ImageHandler struct {
	storage *Storage
	dir     string
}

func NewImageHandler(storage *Storage) *ImageHandler {
	dir, _ := os.UserConfigDir()
	imgDir := filepath.Join(dir, "clipflow", "images")
	os.MkdirAll(imgDir, 0755)
	return &ImageHandler{storage: storage, dir: imgDir}
}

func (ih *ImageHandler) CheckAndSave() string {
	avail, _, _ := procIsClipboardFormatAvailable.Call(CF_DIB)
	if avail == 0 {
		return ""
	}

	procOpenClipboard.Call(0)
	defer procCloseClipboard.Call()

	handle, _, _ := procGetClipboardData.Call(CF_DIB)
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

	data := make([]byte, size)
	copy(data, unsafe.Slice((*byte)(unsafe.Pointer(ptr)), size))

	img, err := decodeDIB(data)
	if err != nil {
		return ""
	}

	id := ih.storage.NextID
	filename := filepath.Join(ih.dir, "img_"+itoa(id)+".png")

	f, err := os.Create(filename)
	if err != nil {
		return ""
	}
	defer f.Close()

	err = png.Encode(f, img)
	if err != nil {
		return ""
	}

	return filename
}

func decodeDIB(data []byte) (image.Image, error) {
	if len(data) < 40 {
		return nil, os.ErrInvalid
	}

	headerSize := binary.LittleEndian.Uint32(data[0:4])

	width := int(binary.LittleEndian.Uint32(data[4:8]))
	height := int(binary.LittleEndian.Uint32(data[8:12]))

	bitCount := binary.LittleEndian.Uint16(data[14:16])
	compression := binary.LittleEndian.Uint32(data[16:20])

	var pixelDataOffset int
	if headerSize >= 40 {
		pixelDataOffset = int(headerSize)
	} else {
		pixelDataOffset = 40
	}

	if width <= 0 || height == 0 {
		return nil, os.ErrInvalid
	}

	absHeight := height
	if height < 0 {
		absHeight = -height
	}

	switch bitCount {
	case 24:
		return decode24BitDIB(data, pixelDataOffset, width, absHeight, height > 0)
	case 32:
		return decode32BitDIB(data, pixelDataOffset, width, absHeight, height > 0)
	default:
		_ = compression
		return decodeFallback(data, width, absHeight, height > 0, bitCount)
	}
}

func decode24BitDIB(data []byte, offset, width, height int, topDown bool) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	rowSize := ((width*24 + 31) / 32) * 4

	for y := 0; y < height; y++ {
		srcY := y
		if !topDown {
			srcY = height - 1 - y
		}
		rowStart := offset + srcY*rowSize
		for x := 0; x < width; x++ {
			pixelStart := rowStart + x*3
			if pixelStart+2 >= len(data) {
				continue
			}
			b := data[pixelStart]
			g := data[pixelStart+1]
			r := data[pixelStart+2]
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img, nil
}

func decode32BitDIB(data []byte, offset, width, height int, topDown bool) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	rowSize := width * 4

	for y := 0; y < height; y++ {
		srcY := y
		if !topDown {
			srcY = height - 1 - y
		}
		rowStart := offset + srcY*rowSize
		for x := 0; x < width; x++ {
			pixelStart := rowStart + x*4
			if pixelStart+3 >= len(data) {
				continue
			}
			b := data[pixelStart]
			g := data[pixelStart+1]
			r := data[pixelStart+2]
			a := data[pixelStart+3]
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	return img, nil
}

func decodeFallback(data []byte, width, height int, topDown bool, bitCount uint16) (image.Image, error) {
	_ = bitCount
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	return img, nil
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func WriteClipboardPNG(data []byte) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	rowSize := (w*32 + 31) / 32 * 4
	dibSize := 40 + h*rowSize
	dib := make([]byte, dibSize)

	binary.LittleEndian.PutUint32(dib[0:4], 40)
	binary.LittleEndian.PutUint32(dib[4:8], uint32(w))
	binary.LittleEndian.PutUint32(dib[8:12], uint32(h))
	binary.LittleEndian.PutUint16(dib[12:14], 1)
	binary.LittleEndian.PutUint16(dib[14:16], 32)
	binary.LittleEndian.PutUint32(dib[16:20], 0)

	pixelOffset := 40
	for y := 0; y < h; y++ {
		srcY := h - 1 - y
		rowStart := pixelOffset + srcY*rowSize
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			px := rowStart + x*4
			dib[px] = byte(b >> 8)
			dib[px+1] = byte(g >> 8)
			dib[px+2] = byte(r >> 8)
			dib[px+3] = byte(a >> 8)
		}
	}

	procOpenClipboard.Call(0)
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()

	handle, _, _ := procGlobalAlloc.Call(GMEM_MOVABLE, uintptr(len(dib)))
	if handle == 0 {
		return syscall.GetLastError()
	}

	ptr, _, _ := procGlobalLock.Call(handle)
	if ptr == 0 {
		return syscall.GetLastError()
	}

	copy(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(dib)), dib)
	procGlobalUnlock.Call(handle)
	procSetClipboardData.Call(CF_DIB, handle)
	return nil
}
