// Package imageinput inspects bounded still-image metadata without allocating
// pixels. Originals remain caller-owned; metadata never supplies paths or code.
package imageinput

import (
	"bytes"
	"encoding/binary"
	"errors"
)

var ErrMetadata = errors.New("invalid or ambiguous image metadata")
var ErrAnimation = errors.New("animated or multi-picture images require an explicit still frame")

// Orientation returns the TIFF/Exif orientation of the primary image (1..8).
// Thumbnail IFDs and unrelated Exif fields are not interpreted. Duplicate Exif
// payloads are rejected rather than letting different decoders choose differently.
func Orientation(data []byte, format string) (int, error) {
	switch format {
	case "jpeg":
		return jpegOrientation(data)
	case "png":
		return pngOrientation(data)
	default:
		return 0, ErrMetadata
	}
}

func jpegOrientation(data []byte) (int, error) {
	if len(data) < 4 || data[0] != 255 || data[1] != 216 {
		return 0, ErrMetadata
	}
	orientation, seen := 1, false
	for at := 2; at < len(data); {
		if data[at] != 255 {
			return 0, ErrMetadata
		}
		for at < len(data) && data[at] == 255 {
			at++
		}
		if at >= len(data) {
			return 0, ErrMetadata
		}
		marker := data[at]
		at++
		if marker == 0xda || marker == 0xd9 { // Pixel decoder validates the remaining stream.
			return orientation, nil
		}
		if marker == 1 || marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		if at+2 > len(data) {
			return 0, ErrMetadata
		}
		length := int(binary.BigEndian.Uint16(data[at:]))
		if length < 2 || length > len(data)-at {
			return 0, ErrMetadata
		}
		payload := data[at+2 : at+length]
		if marker == 0xe2 && bytes.HasPrefix(payload, []byte("MPF\x00")) {
			return 0, ErrAnimation
		}
		if marker == 0xe1 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
			if seen {
				return 0, ErrMetadata
			}
			seen = true
			var err error
			orientation, err = exifOrientation(payload[6:])
			if err != nil {
				return 0, err
			}
		}
		at += length
	}
	return 0, ErrMetadata
}

func pngOrientation(data []byte) (int, error) {
	if len(data) < 8 || !bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return 0, ErrMetadata
	}
	orientation, seen := 1, false
	for at := 8; at <= len(data)-12; {
		length := uint64(binary.BigEndian.Uint32(data[at:]))
		if length > uint64(len(data)-at-12) {
			return 0, ErrMetadata
		}
		end := at + 12 + int(length)
		switch string(data[at+4 : at+8]) {
		case "acTL", "fcTL", "fdAT":
			return 0, ErrAnimation
		case "eXIf":
			if seen {
				return 0, ErrMetadata
			}
			seen = true
			var err error
			orientation, err = exifOrientation(data[at+8 : end-4])
			if err != nil {
				return 0, err
			}
		case "IEND":
			if length != 0 || end != len(data) {
				return 0, ErrMetadata
			}
			return orientation, nil
		}
		at = end
	}
	return 0, ErrMetadata
}

func exifOrientation(data []byte) (int, error) {
	if len(data) < 8 {
		return 0, ErrMetadata
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0, ErrMetadata
	}
	if order.Uint16(data[2:]) != 42 {
		return 0, ErrMetadata
	}
	offset := uint64(order.Uint32(data[4:]))
	if offset < 8 || offset+2 > uint64(len(data)) {
		return 0, ErrMetadata
	}
	count := uint64(order.Uint16(data[offset:]))
	// A primary image directory is bounded independently of the encoded file.
	if count > 512 || offset+2+12*count+4 > uint64(len(data)) {
		return 0, ErrMetadata
	}
	orientation, seen := 1, false
	for i := uint64(0); i < count; i++ {
		entry := data[offset+2+i*12:]
		if order.Uint16(entry) != 0x112 {
			continue
		}
		if seen || order.Uint16(entry[2:]) != 3 || order.Uint32(entry[4:]) != 1 {
			return 0, ErrMetadata
		}
		seen = true
		orientation = int(order.Uint16(entry[8:]))
		if orientation < 1 || orientation > 8 {
			return 0, ErrMetadata
		}
	}
	return orientation, nil
}
