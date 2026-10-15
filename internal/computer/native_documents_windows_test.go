//go:build windows && amd64

package computer

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	ole "github.com/go-ole/go-ole"
)

func TestDocumentWindowsBackendNegotiatesDocumentContract(t *testing.T) {
	if features := ProviderNativeFeatures(&NativeWindows{}); features.Documents != NativeDocumentsVersion {
		t.Fatal("document backend was not negotiated", features)
	}
}

func TestDocumentWindowsTreeComparisonIgnoresOrderButNotChangedRelationships(t *testing.T) {
	before := documentTreeFixture()
	after := documentTreeFixture()
	after[2], after[3] = after[3], after[2]
	if !sameDocumentTree(before, after) {
		t.Fatal("tab ordering changed identity")
	}
	for _, mutate := range []func([]documentNode){
		func(n []documentNode) { n[2].selected = false },
		func(n []documentNode) { n[2].controls = nil },
		func(n []documentNode) { n[5].parent = "window" },
		func(n []documentNode) { n[5].protected = true },
		func(n []documentNode) { n[5].visible = false },
	} {
		changed := documentTreeFixture()
		mutate(changed)
		if sameDocumentTree(before, changed) {
			t.Fatal("changed provider state accepted")
		}
	}
}

// A property VARIANT can contain UIA's reserved IUnknown, not an element array.
// Fake COM objects exercise the interface check without opening any app or
// acquiring desktop/input access. Calling an array vtable on this sentinel is a
// protocol error and, with real UIA, could crash the owned worker.
func TestDocumentControllerForDoesNotCastReservedUnknownToArray(t *testing.T) {
	arrayCalls := 0
	unknownTable := []uintptr{
		syscall.NewCallback(func(_ uintptr, _ uintptr, out uintptr) uintptr {
			*(**ole.IUnknown)(unsafe.Pointer(out)) = nil
			return 0x80004002 // E_NOINTERFACE
		}),
		syscall.NewCallback(func(uintptr) uintptr { return 2 }),
		syscall.NewCallback(func(uintptr) uintptr { return 1 }),
		syscall.NewCallback(func(uintptr, uintptr) uintptr { arrayCalls++; return 0x80004002 }),
	}
	unknown := ole.IUnknown{RawVTable: (*interface{})(unsafe.Pointer(&unknownTable[0]))}
	elementTable := make([]uintptr, 11)
	elementTable[10] = syscall.NewCallback(func(_ uintptr, property uintptr, out uintptr) uintptr {
		if property != 30104 {
			return 0x80070057
		}
		value := (*ole.VARIANT)(unsafe.Pointer(out))
		*value = ole.NewVariant(ole.VT_UNKNOWN, int64(uintptr(unsafe.Pointer(&unknown))))
		return 0
	})
	element := ole.IUnknown{RawVTable: (*interface{})(unsafe.Pointer(&elementTable[0]))}
	panes, err := uiaControlledPanes(&element)
	if err != nil || len(panes) != 0 || arrayCalls != 0 {
		t.Fatal("reserved value used as array", panes, arrayCalls, err)
	}
	runtime.KeepAlive(unknownTable)
	runtime.KeepAlive(elementTable)
	runtime.KeepAlive(unknown)
}
