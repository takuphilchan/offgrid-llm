package imageinput

// OriginalBounds maps an upright half-open pixel rectangle back to the encoded
// image's pixel grid. It inverts Orient, including mirrored EXIF orientations.
func OriginalBounds(box [4]int, width, height, orientation int) ([4]int, error) {
	if width < 1 || height < 1 || orientation < 1 || orientation > 8 {
		return [4]int{}, ErrMetadata
	}
	ow, oh := width, height
	if orientation >= 5 {
		ow, oh = height, width
	}
	if box[0] < 0 || box[1] < 0 || box[2] <= box[0] || box[3] <= box[1] || box[2] > ow || box[3] > oh {
		return [4]int{}, ErrMetadata
	}
	x0, y0, x1, y1 := box[0], box[1], box[2], box[3]
	switch orientation {
	case 2:
		return [4]int{width - x1, y0, width - x0, y1}, nil
	case 3:
		return [4]int{width - x1, height - y1, width - x0, height - y0}, nil
	case 4:
		return [4]int{x0, height - y1, x1, height - y0}, nil
	case 5:
		return [4]int{y0, x0, y1, x1}, nil
	case 6:
		return [4]int{y0, height - x1, y1, height - x0}, nil
	case 7:
		return [4]int{width - y1, height - x1, width - y0, height - x0}, nil
	case 8:
		return [4]int{width - y1, x0, width - y0, x1}, nil
	}
	return box, nil
}
