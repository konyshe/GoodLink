//go:build windows

package udpbatch

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestUSOCMsgLayout(t *testing.T) {
	switch unsafe.Sizeof(uintptr(0)) {
	case 8:
		if cmsgSpace(4) != 24 || cmsgLen(4) != 20 || cmsgDataOff() != 16 {
			t.Fatalf("amd64 cmsg space=%d len=%d off=%d", cmsgSpace(4), cmsgLen(4), cmsgDataOff())
		}
	case 4:
		if cmsgSpace(4) != 16 || cmsgLen(4) != 16 || cmsgDataOff() != 12 {
			t.Fatalf("386 cmsg space=%d len=%d off=%d", cmsgSpace(4), cmsgLen(4), cmsgDataOff())
		}
	default:
		t.Fatalf("unexpected pointer size %d", unsafe.Sizeof(uintptr(0)))
	}

	w := &windowsWriter{}
	b := w.control(1234)
	if uintptr(unsafe.Pointer(&b[0]))%unsafe.Alignof(windows.WSACMSGHDR{}) != 0 {
		t.Fatal("cmsg misaligned")
	}
	hdr := (*windows.WSACMSGHDR)(unsafe.Pointer(&b[0]))
	if hdr.Level != windows.IPPROTO_UDP || hdr.Type != windows.UDP_SEND_MSG_SIZE {
		t.Fatalf("hdr level=%d type=%d", hdr.Level, hdr.Type)
	}
	if hdr.Len != uintptr(cmsgLen(4)) {
		t.Fatalf("hdr.Len=%d want %d", hdr.Len, cmsgLen(4))
	}
	if len(b) != cmsgSpace(4) {
		t.Fatalf("buf len=%d want %d", len(b), cmsgSpace(4))
	}
	got := *(*uint32)(unsafe.Add(unsafe.Pointer(&b[0]), cmsgDataOff()))
	if got != 1234 {
		t.Fatalf("seg=%d", got)
	}
}
