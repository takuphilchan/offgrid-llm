package verification

import (
	"golang.org/x/sys/windows"
	"os"
)

func openRead(root *os.Root, name string) (*os.File, error) { return root.Open(name) }

func singleLink(f *os.File) bool {
	var info windows.ByHandleFileInformation
	return windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info) == nil && info.NumberOfLinks == 1 && info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0
}
