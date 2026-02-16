package mathutil

// Mat3 is a 3×3 matrix stored row-major: [r0c0, r0c1, r0c2, r1c0, ...].
// Value type for zero heap allocation.
type Mat3 [9]float64

func Mat3Identity() Mat3 {
	return Mat3{1, 0, 0, 0, 1, 0, 0, 0, 1}
}

func Mat3Diag(x, y, z float64) Mat3 {
	return Mat3{x, 0, 0, 0, y, 0, 0, 0, z}
}

// Mat3Mul returns a × b.
func Mat3Mul(a, b Mat3) Mat3 {
	var m Mat3
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			m[r*3+c] = a[r*3+0]*b[0*3+c] + a[r*3+1]*b[1*3+c] + a[r*3+2]*b[2*3+c]
		}
	}
	return m
}

// MulVec3 returns M × v.
func (m Mat3) MulVec3(v Vec3) Vec3 {
	return Vec3{
		m[0]*v[0] + m[1]*v[1] + m[2]*v[2],
		m[3]*v[0] + m[4]*v[1] + m[5]*v[2],
		m[6]*v[0] + m[7]*v[1] + m[8]*v[2],
	}
}

func (m Mat3) Det() float64 {
	return m[0]*(m[4]*m[8]-m[5]*m[7]) -
		m[1]*(m[3]*m[8]-m[5]*m[6]) +
		m[2]*(m[3]*m[7]-m[4]*m[6])
}

func (m Mat3) Inverse() Mat3 {
	d := m.Det()
	if d == 0 {
		return Mat3Identity()
	}
	invD := 1.0 / d
	return Mat3{
		(m[4]*m[8] - m[5]*m[7]) * invD,
		(m[2]*m[7] - m[1]*m[8]) * invD,
		(m[1]*m[5] - m[2]*m[4]) * invD,
		(m[5]*m[6] - m[3]*m[8]) * invD,
		(m[0]*m[8] - m[2]*m[6]) * invD,
		(m[2]*m[3] - m[0]*m[5]) * invD,
		(m[3]*m[7] - m[4]*m[6]) * invD,
		(m[1]*m[6] - m[0]*m[7]) * invD,
		(m[0]*m[4] - m[1]*m[3]) * invD,
	}
}

func (m Mat3) Transpose() Mat3 {
	return Mat3{
		m[0], m[3], m[6],
		m[1], m[4], m[7],
		m[2], m[5], m[8],
	}
}
