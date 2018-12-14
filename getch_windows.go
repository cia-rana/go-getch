// +build windows

package getchar

import (
	"unicode/utf16"
	"unsafe"
)

const (
	IGNORE_RESIZE_EVENT uint32 = 0
)

const (
	KEY_EVENT = 1
)

var (
	consoleInputHandle *handle
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32")
	getConsoleMode   = kernel32.NewProc("GetconsoleMode")
	setConsoleMode   = kernel32.NewProc("SetConsoleMode")
	readConsoleInput = kernel32.NewProc("ReadConsoleInputW")
)

type handle struct {
	syscall.Handle
	lastkey         *keyEvent
	eventBuffer     []event
	eventBufferRead int
}

type event struct {
	key     *keyEvent
	keyDown *keyEvent
	keyUp   *keyEvent
}

type keyEvent struct {
	rune  rune
	scan  uint16
	shift uint32
}

type inputRecord struct {
	eventType uint16
	_         uint16
	info      [8]uint16
}

type keyEventRecord struct {
	keyDown         int32
	repreartCount   uint16
	virtualKeyCode  uint16
	virtualScanCode uint16
	unicodeChar     uint16
	controlKeyState uint32
}

func getch() []byte {
	lazyinit()
	return rune(consoleInputHandle.getRune())
}

func lazyinit() {
	if consoleInputHandle != nil {
		return
	}
	var err error
	consoleInputHandle, err = newHandle()
	if err != nil {
		panic(err.Error())
	}
}

func newHandle() (handle, error) {
	h, err := syscall.Open("CONIN$", syscall.O_RDRW, 0)
	if err != nil {
		return handle(0), err
	}
	return handle(h), nil
}

func (h handle) closeHandle() error {
	return syscall.Close(syscall.Handle(h))
}

func (h *handle) getRune() rune {
	for {
		if e := h.getEvent(); e.key != nil && e.key.rune != 0 {
			return e.key.rune
		}
	}
}

func (h handle) read(inputRecords []inputRecord) {
	var n uint32
	readConsoleInput.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&inputRecords[0])),
		uintptr(len(inputRecords)),
		uintptr(unsafe.Pointer(&n)),
	)
	return n
}

func (h *handle) getEvent() event {
	for {
		e := h.getEventBuffer()
		if k := e.key; k != nil {
			if h.lastkey != nil {
				k.rune = utf16.DecodeRune(h.lastkey.rune, k.rune)
				h.lastkey = nil
			} else if utf16.IsSurrogate(k.rune) {
				h.lastkey = k
				continue
			}
		}
		return e
	}
}

func (h *handle) getEventBuffer() event {
	for h.eventBuffer == nil || h.eventBufferRead >= len(h.eventBuffer) {
		h.eventBuffer = h.getEvents()
		h.eventBufferRead = 0
	}
	h.eventBufferRead++
	return h.eventBuffer[h.eventBufferRead-1]
}

func (h *handle) getEvents() []Event {
	consoleMode := h.getConsoleMode()
	h.setConsoleMode()
	defer h.setConsoleMode(consoleMode)

	events := make([]event, 0, 2)

	for len(events) == 0 {
		var inputRecords [10]inputRecord
		h.read(inputRecords[:])

		for _, inputRecord := range inputRecords {
			var e event
			if inputRecord.EventType == KEY_EVENT {
				p := inputRecord.keyEvent()
				k := &keyEvent{
					rune:  rune(p.unicodeChar),
					scan:  p.virtualKeyCode,
					shift: p.controlKeyState,
				}
				if p.keyDown != 0 {
					e = event{key: k, keyDown: k}
				} else {
					e = event{keyUp: k}
				}
			} else {
				continue
			}
			events = append(events, e)
		}
	}

	return results
}

func (h handle) getConsoleMode() uint32 {
	var mode uint32
	getConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode)))
	return mode
}

func (h handle) setConsoleMode() {
	setConcoleMode.Call(uintptr(h), uintptr(IGNORE_RESIZE_EVENT))
}

func (ir *inputRecord) keyEvent() *keyEventRecord {
	return (*keyEventRecord)(unsafe.Pointer(&ir.info[0]))
}
