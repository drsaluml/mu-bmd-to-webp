package mathutil

import "math"

// Vec3 is a 3-component vector (value type, stack-allocated).
type Vec3 [3]float64

func (a Vec3) Add(b Vec3) Vec3 {
	return Vec3{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

func (a Vec3) Sub(b Vec3) Vec3 {
	return Vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

func (v Vec3) Scale(s float64) Vec3 {
	return Vec3{v[0] * s, v[1] * s, v[2] * s}
}

func (a Vec3) Dot(b Vec3) float64 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func (v Vec3) Len() float64 {
	return math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
}

func (v Vec3) Normalize() Vec3 {
	l := v.Len()
	if l < 1e-12 {
		return Vec3{}
	}
	return Vec3{v[0] / l, v[1] / l, v[2] / l}
}
