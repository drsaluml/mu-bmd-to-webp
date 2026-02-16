package mathutil

import "math"

// RotX returns a 3×3 rotation matrix around the X axis. Angle in radians.
func RotX(a float64) Mat3 {
	c, s := math.Cos(a), math.Sin(a)
	return Mat3{
		1, 0, 0,
		0, c, -s,
		0, s, c,
	}
}

// RotY returns a 3×3 rotation matrix around the Y axis.
func RotY(a float64) Mat3 {
	c, s := math.Cos(a), math.Sin(a)
	return Mat3{
		c, 0, s,
		0, 1, 0,
		-s, 0, c,
	}
}

// RotZ returns a 3×3 rotation matrix around the Z axis.
func RotZ(a float64) Mat3 {
	c, s := math.Cos(a), math.Sin(a)
	return Mat3{
		c, -s, 0,
		s, c, 0,
		0, 0, 1,
	}
}

// Deg2Rad converts degrees to radians.
func Deg2Rad(d float64) float64 {
	return d * math.Pi / 180
}
