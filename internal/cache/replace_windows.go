//go:build windows

package cache

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	movefileReplaceExisting = 0x1
	movefileWriteThrough    = 0x8
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procMoveFileEx = kernel32.NewProc("MoveFileExW")
)

func replaceFile(source string, target string) error {
	sourcePtr, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	ret, _, callErr := procMoveFileEx.Call(
		uintptr(unsafe.Pointer(sourcePtr)),
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(movefileReplaceExisting|movefileWriteThrough),
	)
	if ret == 0 {
		return fmt.Errorf("replace cache file: %w", callErr)
	}
	return nil
}
