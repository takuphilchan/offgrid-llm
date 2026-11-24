package storage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

const maxBackupDirectoryBytes int64 = 64 << 20

// Validate the ZIP/ZIP64 central-directory envelope and actual entry count
// before archive/zip allocates its file inventory. This is a bounds preflight;
// archive/zip still owns decompression and complete ZIP consistency checks.
func validateBackupZIPDirectory(reader io.ReaderAt, size int64) error {
	invalid := errors.New("backup ZIP directory is invalid or exceeds limits")
	if size < 22 || size > maxBackupArchiveBytes {
		return invalid
	}
	tail := make([]byte, min(size, 65535+22))
	tailStart := size - int64(len(tail))
	if _, err := reader.ReadAt(tail, tailStart); err != nil {
		return err
	}
	end := -1
	for start := len(tail) - 22; start >= 0; start-- {
		if bytes.Equal(tail[start:start+4], []byte{'P', 'K', 5, 6}) && start+22+int(binary.LittleEndian.Uint16(tail[start+20:start+22])) == len(tail) {
			end = start
			break
		}
	}
	if end < 0 {
		return invalid
	}
	record := tail[end:]
	if binary.LittleEndian.Uint16(record[4:6]) != 0 || binary.LittleEndian.Uint16(record[6:8]) != 0 {
		return invalid
	}
	count := uint64(binary.LittleEndian.Uint16(record[10:12]))
	if uint64(binary.LittleEndian.Uint16(record[8:10])) != count {
		return invalid
	}
	directorySize := uint64(binary.LittleEndian.Uint32(record[12:16]))
	offset := uint64(binary.LittleEndian.Uint32(record[16:20]))
	directoryEnd := tailStart + int64(end)
	if count == 65535 || directorySize == 0xffffffff || offset == 0xffffffff {
		if directoryEnd < 20 {
			return invalid
		}
		var locator [20]byte
		if _, err := reader.ReadAt(locator[:], directoryEnd-20); err != nil {
			return err
		}
		if binary.LittleEndian.Uint32(locator[:4]) != 0x07064b50 || binary.LittleEndian.Uint32(locator[4:8]) != 0 || binary.LittleEndian.Uint32(locator[16:20]) != 1 {
			return invalid
		}
		zip64Offset := binary.LittleEndian.Uint64(locator[8:16])
		if zip64Offset > uint64(directoryEnd-20) || uint64(directoryEnd-20)-zip64Offset < 56 {
			return invalid
		}
		var extended [56]byte
		if _, err := reader.ReadAt(extended[:], int64(zip64Offset)); err != nil {
			return err
		}
		extendedSize := binary.LittleEndian.Uint64(extended[4:12])
		if binary.LittleEndian.Uint32(extended[:4]) != 0x06064b50 || extendedSize < 44 || extendedSize > uint64(maxBackupDirectoryBytes) || zip64Offset+12+extendedSize != uint64(directoryEnd-20) {
			return invalid
		}
		if binary.LittleEndian.Uint32(extended[16:20]) != 0 || binary.LittleEndian.Uint32(extended[20:24]) != 0 {
			return invalid
		}
		count = binary.LittleEndian.Uint64(extended[32:40])
		if binary.LittleEndian.Uint64(extended[24:32]) != count {
			return invalid
		}
		directorySize, offset = binary.LittleEndian.Uint64(extended[40:48]), binary.LittleEndian.Uint64(extended[48:56])
		directoryEnd = int64(zip64Offset)
	}
	if count == 0 || count > maxBackupFiles+1 || directorySize > uint64(maxBackupDirectoryBytes) || offset > uint64(directoryEnd) || directorySize != uint64(directoryEnd)-offset {
		return invalid
	}
	var header [46]byte
	position := offset
	for i := uint64(0); i < count; i++ {
		if uint64(directoryEnd)-position < uint64(len(header)) {
			return invalid
		}
		if _, err := reader.ReadAt(header[:], int64(position)); err != nil {
			return err
		}
		if binary.LittleEndian.Uint32(header[:4]) != 0x02014b50 || binary.LittleEndian.Uint16(header[34:36]) != 0 {
			return invalid
		}
		length := uint64(len(header)) + uint64(binary.LittleEndian.Uint16(header[28:30])) + uint64(binary.LittleEndian.Uint16(header[30:32])) + uint64(binary.LittleEndian.Uint16(header[32:34]))
		if length > uint64(directoryEnd)-position {
			return invalid
		}
		position += length
	}
	if position != uint64(directoryEnd) {
		return invalid
	}
	return nil
}
