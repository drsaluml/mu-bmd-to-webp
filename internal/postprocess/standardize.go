package postprocess

import (
	"image"
	"image/color"
	"math"

	"mu-bmd-renderer/internal/mathutil"

	"golang.org/x/image/draw"
)

// CropAndCenter crops to the bounding box of non-transparent pixels, then scales and centers.
// Used when PCA standardization is disabled (standardize: false).
func CropAndCenter(img *image.NRGBA, size int, fillRatio float64) *image.NRGBA {
	cropped := cropAlpha(img)
	return scaleAndCenter(cropped, size, fillRatio)
}

// StandardizeImage rotates, scales, and centers the item image using PCA alignment.
func StandardizeImage(img *image.NRGBA, size int, targetAngleDeg, fillRatio float64, forceFlip bool) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// Collect non-transparent pixel coordinates
	var xs, ys []float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if img.Pix[y*img.Stride+x*4+3] > 0 {
				xs = append(xs, float64(x))
				ys = append(ys, float64(y))
			}
		}
	}

	if len(xs) < 10 {
		return img
	}

	// PCA: compute covariance matrix of pixel positions
	n := float64(len(xs))
	var meanX, meanY float64
	for i := range xs {
		meanX += xs[i]
		meanY += ys[i]
	}
	meanX /= n
	meanY /= n

	var covXX, covXY, covYY float64
	for i := range xs {
		dx := xs[i] - meanX
		dy := ys[i] - meanY
		covXX += dx * dx
		covXY += dx * dy
		covYY += dy * dy
	}
	covXX /= n
	covXY /= n
	covYY /= n

	// Eigendecomposition of 2×2 symmetric matrix
	_, _, evec1, _ := mathutil.Eigen2x2Sym(covXX, covXY, covYY)

	// Current PCA angle in image coordinates (atan2(y, x), y-down)
	currentAngle := math.Atan2(evec1[1], evec1[0]) * 180.0 / math.Pi

	// Target angle in image space: negate math convention (y-up → y-down)
	targetImg := -targetAngleDeg

	// Rotation needed: PIL rotate(θ) rotates CCW, new_angle = old_angle - θ
	// So θ = old_angle - target_angle
	pilRotate := currentAngle - targetImg
	// Normalize to [-90, 90] — PCA eigenvector has 180° ambiguity
	for pilRotate > 90 {
		pilRotate -= 180
	}
	for pilRotate < -90 {
		pilRotate += 180
	}

	// Rotate image
	rotated := rotateImage(img, pilRotate)

	// Auto-detect orientation on the rotated image:
	// Project rotated pixels along target direction, check which half is wider
	needFlip := detectFlipRotated(rotated, targetImg)
	if forceFlip {
		needFlip = !needFlip
	}
	if needFlip {
		rotated = rotate180(rotated)
	}

	// Crop to bounding box of non-transparent pixels
	cropped := cropAlpha(rotated)

	// Scale to fill_ratio of canvas and center
	return scaleAndCenter(cropped, size, fillRatio)
}

// detectFlipRotated checks orientation on the already-rotated image.
// Projects pixels along the target angle direction and compares
// perpendicular spread of top-left vs bottom-right halves.
// Matches Python's approach: std-based spread comparison on rotated pixels.
func detectFlipRotated(rotated *image.NRGBA, targetImgDeg float64) bool {
	b := rotated.Bounds()
	w, h := b.Dx(), b.Dy()

	// Collect non-transparent pixel coordinates
	var rxs, rys []float64
	var minX, maxX, minY, maxY float64
	first := true
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if rotated.Pix[y*rotated.Stride+x*4+3] > 0 {
				fx, fy := float64(x), float64(y)
				rxs = append(rxs, fx)
				rys = append(rys, fy)
				if first {
					minX, maxX = fx, fx
					minY, maxY = fy, fy
				}
				if fx < minX {
					minX = fx
				}
				if fx > maxX {
					maxX = fx
				}
				if fy < minY {
					minY = fy
				}
				if fy > maxY {
					maxY = fy
				}
				first = false
			}
		}
	}

	if len(rxs) < 20 {
		return false
	}

	// Center of bounding box
	cx := (minX + maxX) / 2.0
	cy := (minY + maxY) / 2.0

	// Target direction vector in image space
	rad := targetImgDeg * math.Pi / 180.0
	diagX := math.Cos(rad)
	diagY := math.Sin(rad)

	// Project pixels along target direction, compute perpendicular spread per half
	// neg_half = top-left (proj < 0), pos_half = bottom-right (proj >= 0)
	var negPerps, posPerps []float64
	for i := range rxs {
		dx := rxs[i] - cx
		dy := rys[i] - cy
		proj := dx*diagX + dy*diagY
		perp := -dx*diagY + dy*diagX

		if proj < 0 {
			negPerps = append(negPerps, perp)
		} else {
			posPerps = append(posPerps, perp)
		}
	}

	spreadTL := stddev(negPerps)
	spreadBR := stddev(posPerps)

	// Wider end should be at top-left; flip if bottom-right is wider
	return spreadBR > spreadTL*1.2
}

func stddev(vals []float64) float64 {
	if len(vals) < 10 {
		return 0
	}
	n := float64(len(vals))
	var sum, sum2 float64
	for _, v := range vals {
		sum += v
		sum2 += v * v
	}
	mean := sum / n
	variance := sum2/n - mean*mean
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}

func rotateImage(img *image.NRGBA, angleDeg float64) *image.NRGBA {
	if math.Abs(angleDeg) < 0.5 {
		return img
	}

	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	rad := angleDeg * math.Pi / 180.0
	cos := math.Abs(math.Cos(rad))
	sin := math.Abs(math.Sin(rad))

	// Expanded canvas to fit rotated image
	newW := int(math.Ceil(w*cos + h*sin))
	newH := int(math.Ceil(w*sin + h*cos))

	dst := image.NewNRGBA(image.Rect(0, 0, newW, newH))

	// Affine transform: rotate around center
	cx, cy := w/2, h/2
	ncx, ncy := float64(newW)/2, float64(newH)/2
	cosA := math.Cos(rad)
	sinA := math.Sin(rad)

	// For each destination pixel, find source pixel (inverse mapping)
	for dy := 0; dy < newH; dy++ {
		for dx := 0; dx < newW; dx++ {
			// Translate to center, inverse rotate, translate back
			rx := float64(dx) - ncx
			ry := float64(dy) - ncy
			sx := rx*cosA + ry*sinA + cx
			sy := -rx*sinA + ry*cosA + cy

			// Bilinear interpolation
			x0 := int(math.Floor(sx))
			y0 := int(math.Floor(sy))
			x1 := x0 + 1
			y1 := y0 + 1
			fx := sx - float64(x0)
			fy := sy - float64(y0)

			if x0 < 0 || y0 < 0 || x1 >= b.Dx() || y1 >= b.Dy() {
				continue
			}

			// Sample 4 corners
			c00 := samplePix(img, x0, y0)
			c10 := samplePix(img, x1, y0)
			c01 := samplePix(img, x0, y1)
			c11 := samplePix(img, x1, y1)

			r := lerp4(c00[0], c10[0], c01[0], c11[0], fx, fy)
			g := lerp4(c00[1], c10[1], c01[1], c11[1], fx, fy)
			bv := lerp4(c00[2], c10[2], c01[2], c11[2], fx, fy)
			a := lerp4(c00[3], c10[3], c01[3], c11[3], fx, fy)

			i := dst.PixOffset(dx, dy)
			dst.Pix[i] = clamp8(r)
			dst.Pix[i+1] = clamp8(g)
			dst.Pix[i+2] = clamp8(bv)
			dst.Pix[i+3] = clamp8(a)
		}
	}

	return dst
}

func samplePix(img *image.NRGBA, x, y int) [4]float64 {
	i := img.PixOffset(x, y)
	return [4]float64{
		float64(img.Pix[i]),
		float64(img.Pix[i+1]),
		float64(img.Pix[i+2]),
		float64(img.Pix[i+3]),
	}
}

func lerp4(v00, v10, v01, v11, fx, fy float64) float64 {
	return v00*(1-fx)*(1-fy) + v10*fx*(1-fy) + v01*(1-fx)*fy + v11*fx*fy
}

func rotate180(img *image.NRGBA) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := img.PixOffset(x, y)
			di := dst.PixOffset(w-1-x, h-1-y)
			copy(dst.Pix[di:di+4], img.Pix[si:si+4])
		}
	}
	return dst
}

func cropAlpha(img *image.NRGBA) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	minX, minY := w, h
	maxX, maxY := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if img.Pix[y*img.Stride+x*4+3] > 0 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	if maxX <= minX || maxY <= minY {
		return img
	}

	cropW := maxX - minX + 1
	cropH := maxY - minY + 1
	cropped := image.NewNRGBA(image.Rect(0, 0, cropW, cropH))
	for y := 0; y < cropH; y++ {
		srcOff := (minY+y)*img.Stride + minX*4
		dstOff := y * cropped.Stride
		copy(cropped.Pix[dstOff:dstOff+cropW*4], img.Pix[srcOff:srcOff+cropW*4])
	}
	return cropped
}

func scaleAndCenter(img *image.NRGBA, canvasSize int, fillRatio float64) *image.NRGBA {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return image.NewNRGBA(image.Rect(0, 0, canvasSize, canvasSize))
	}

	// Scale to fit within fillRatio of canvas
	maxDim := float64(canvasSize) * fillRatio
	scaleF := maxDim / math.Max(float64(srcW), float64(srcH))
	newW := int(float64(srcW)*scaleF + 0.5)
	newH := int(float64(srcH)*scaleF + 0.5)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	// Resize
	scaled := image.NewNRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Src, nil)

	// Center on canvas
	canvas := image.NewNRGBA(image.Rect(0, 0, canvasSize, canvasSize))
	offX := (canvasSize - newW) / 2
	offY := (canvasSize - newH) / 2
	for y := 0; y < newH; y++ {
		srcOff := y * scaled.Stride
		dstOff := (offY+y)*canvas.Stride + offX*4
		if offY+y >= 0 && offY+y < canvasSize {
			copyLen := newW * 4
			if offX+newW > canvasSize {
				copyLen = (canvasSize - offX) * 4
			}
			if offX >= 0 && copyLen > 0 {
				copy(canvas.Pix[dstOff:dstOff+copyLen], scaled.Pix[srcOff:srcOff+copyLen])
			}
		}
	}

	// Set background to transparent (already zeroed)
	_ = color.NRGBA{}
	return canvas
}

// FlipHorizontal mirrors an image left-to-right.
func FlipHorizontal(img *image.NRGBA) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcOff := y * img.Stride
		dstOff := y * out.Stride
		for x := 0; x < w; x++ {
			mx := w - 1 - x
			si := srcOff + mx*4
			di := dstOff + x*4
			out.Pix[di] = img.Pix[si]
			out.Pix[di+1] = img.Pix[si+1]
			out.Pix[di+2] = img.Pix[si+2]
			out.Pix[di+3] = img.Pix[si+3]
		}
	}
	return out
}
