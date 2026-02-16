package mathutil

import "math"

// Quat represents a quaternion (x, y, z, w).
type Quat [4]float64

// EulerToQuat converts Euler XYZ (radians) to a quaternion.
// Matches MU Online's bmdAngleToQuaternion function.
func EulerToQuat(rx, ry, rz float64) Quat {
	cx, sx := math.Cos(rx*0.5), math.Sin(rx*0.5)
	cy, sy := math.Cos(ry*0.5), math.Sin(ry*0.5)
	cz, sz := math.Cos(rz*0.5), math.Sin(rz*0.5)

	return Quat{
		sx*cy*cz - cx*sy*sz, // x
		cx*sy*cz + sx*cy*sz, // y
		cx*cy*sz - sx*sy*cz, // z
		cx*cy*cz + sx*sy*sz, // w
	}
}

// QuatToMat3 converts a quaternion to a 3×3 rotation matrix.
func QuatToMat3(q Quat) Mat3 {
	x, y, z, w := q[0], q[1], q[2], q[3]
	xx, yy, zz := x*x, y*y, z*z
	xy, xz, yz := x*y, x*z, y*z
	wx, wy, wz := w*x, w*y, w*z

	return Mat3{
		1 - 2*(yy+zz), 2 * (xy - wz), 2 * (xz + wy),
		2 * (xy + wz), 1 - 2*(xx+zz), 2 * (yz - wx),
		2 * (xz - wy), 2 * (yz + wx), 1 - 2*(xx+yy),
	}
}
