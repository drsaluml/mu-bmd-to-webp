package mathutil

// Mat4 is a 4×4 matrix stored row-major. Used for bone world transforms.
type Mat4 [16]float64

func Mat4Identity() Mat4 {
	return Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// Mat4Mul returns a × b.
func Mat4Mul(a, b Mat4) Mat4 {
	var m Mat4
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			m[r*4+c] = a[r*4+0]*b[0*4+c] + a[r*4+1]*b[1*4+c] +
				a[r*4+2]*b[2*4+c] + a[r*4+3]*b[3*4+c]
		}
	}
	return m
}

// MulPoint transforms a 3D point (w=1) by the 4×4 matrix.
func (m Mat4) MulPoint(v Vec3) Vec3 {
	return Vec3{
		m[0]*v[0] + m[1]*v[1] + m[2]*v[2] + m[3],
		m[4]*v[0] + m[5]*v[1] + m[6]*v[2] + m[7],
		m[8]*v[0] + m[9]*v[1] + m[10]*v[2] + m[11],
	}
}

// FromMat3Translation builds a 4×4 affine matrix from a 3×3 rotation and translation.
func FromMat3Translation(r Mat3, t Vec3) Mat4 {
	return Mat4{
		r[0], r[1], r[2], t[0],
		r[3], r[4], r[5], t[1],
		r[6], r[7], r[8], t[2],
		0, 0, 0, 1,
	}
}

// IsIdentity checks if the matrix is approximately identity.
func (m Mat4) IsIdentity() bool {
	id := Mat4Identity()
	for i := 0; i < 16; i++ {
		d := m[i] - id[i]
		if d > 1e-8 || d < -1e-8 {
			return false
		}
	}
	return true
}
