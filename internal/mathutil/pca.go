package mathutil

import "math"

// Eigen2x2Sym computes eigenvalues and eigenvectors of a 2×2 symmetric matrix:
//
//	| a  b |
//	| b  d |
//
// Returns (eval1, eval2, evec1, evec2) where eval1 >= eval2.
// evec1 is the principal eigenvector (largest eigenvalue).
func Eigen2x2Sym(a, b, d float64) (float64, float64, [2]float64, [2]float64) {
	trace := a + d
	det := a*d - b*b
	disc := trace*trace/4 - det
	if disc < 0 {
		disc = 0
	}
	sqrtDisc := math.Sqrt(disc)

	eval1 := trace/2 + sqrtDisc
	eval2 := trace/2 - sqrtDisc

	var evec1, evec2 [2]float64

	if math.Abs(b) > 1e-12 {
		evec1 = normalize2(eval1-d, b)
		evec2 = normalize2(eval2-d, b)
	} else if a >= d {
		evec1 = [2]float64{1, 0}
		evec2 = [2]float64{0, 1}
	} else {
		evec1 = [2]float64{0, 1}
		evec2 = [2]float64{1, 0}
	}

	return eval1, eval2, evec1, evec2
}

func normalize2(x, y float64) [2]float64 {
	l := math.Sqrt(x*x + y*y)
	if l < 1e-12 {
		return [2]float64{1, 0}
	}
	return [2]float64{x / l, y / l}
}
