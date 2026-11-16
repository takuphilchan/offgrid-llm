// Package tifffixture builds self-authored, uncompressed multipage TIFF fixtures.
package tifffixture

import (
	"encoding/binary"
	"image"
	"image/color"
)

func Encode(images []image.Image, orientations []uint16, order binary.ByteOrder) []byte {
	data := make([]byte, 8)
	if order == binary.BigEndian {
		copy(data, "MM")
	} else {
		copy(data, "II")
	}
	order.PutUint16(data[2:], 42)
	previous := 4
	for i, img := range images {
		if len(data)%2 != 0 {
			data = append(data, 0)
		}
		start := len(data)
		order.PutUint32(data[previous:], uint32(start))
		const n = 11
		data = append(data, make([]byte, 2+12*n+4)...)
		order.PutUint16(data[start:], n)
		previous = start + 2 + 12*n
		bounds := img.Bounds()
		w, h := bounds.Dx(), bounds.Dy()
		orientation := uint32(1)
		if i < len(orientations) {
			orientation = uint32(orientations[i])
		}
		entries := [][3]uint32{{256, 4, uint32(w)}, {257, 4, uint32(h)}, {258, 3, 8}, {259, 3, 1}, {262, 3, 1}, {273, 4, uint32(len(data))}, {274, 3, orientation}, {277, 3, 1}, {278, 4, uint32(h)}, {279, 4, uint32(w * h)}, {284, 3, 1}}
		for j, e := range entries {
			p := start + 2 + j*12
			order.PutUint16(data[p:], uint16(e[0]))
			order.PutUint16(data[p+2:], uint16(e[1]))
			order.PutUint32(data[p+4:], 1)
			if e[1] == 3 {
				order.PutUint16(data[p+8:], uint16(e[2]))
			} else {
				order.PutUint32(data[p+8:], e[2])
			}
		}
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				data = append(data, color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y)
			}
		}
	}
	return data
}

func TwoPages() []byte {
	a, b := image.NewGray(image.Rect(0, 0, 120, 80)), image.NewGray(image.Rect(0, 0, 120, 80))
	for i := range a.Pix {
		a.Pix[i] = 230
		b.Pix[i] = 80
	}
	return Encode([]image.Image{a, b}, nil, binary.LittleEndian)
}
