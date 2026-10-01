package covers

import (
	"image"
	"image/draw"
)

// shrink scales img down so its longer side is size, averaging the source
// pixels under each destination pixel (a box filter: good enough for
// covers, and in the standard library only). Smaller images are returned as
// they are.
func shrink(img image.Image, size int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= size && h <= size {
		return img
	}
	nw, nh := size, size
	if w > h {
		nh = max(1, h*size/w)
	} else {
		nw = max(1, w*size/h)
	}
	src, ok := img.(*image.RGBA)
	if !ok || src.Bounds().Min != (image.Point{}) {
		src = image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		y0, y1 := y*h/nh, max((y+1)*h/nh, y*h/nh+1)
		for x := 0; x < nw; x++ {
			x0, x1 := x*w/nw, max((x+1)*w/nw, x*w/nw+1)
			var sum [4]uint32
			for sy := y0; sy < y1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1; sx++ {
					p := row[sx*4 : sx*4+4]
					sum[0] += uint32(p[0])
					sum[1] += uint32(p[1])
					sum[2] += uint32(p[2])
					sum[3] += uint32(p[3])
				}
			}
			n := uint32((y1 - y0) * (x1 - x0))
			d := dst.Pix[y*dst.Stride+x*4:]
			d[0], d[1], d[2], d[3] = uint8(sum[0]/n), uint8(sum[1]/n), uint8(sum[2]/n), uint8(sum[3]/n)
		}
	}
	return dst
}
