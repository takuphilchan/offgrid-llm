package cli

import (
	"io"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

type USBImporter interface {
	ScanUSBDrive(string) ([]string, error)
	GetModelInfo(string) (string, string)
	ImportAll(string, func(models.ImportProgress)) (int, error)
	ImportModel(string, string, func(models.ImportProgress)) error
}

// ModelTransferDependencies are OS-owned file and interaction capabilities.
// Commands receive them only after selecting the configured model directory.
type ModelTransferDependencies struct {
	Remove         func(string) error
	MkdirAll       func(string, os.FileMode) error
	Open           func(string) (io.ReadCloser, error)
	Create         func(string) (io.WriteCloser, error)
	Stat           func(string) (os.FileInfo, error)
	NewUSBImporter func(string, *models.Registry) USBImporter
	RenderProgress func(int64, int64, float64, int)
	ReloadServer   func() error
	ReloadModel    func(string) error
}
