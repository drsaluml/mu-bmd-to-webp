package bmd

// Triangle holds polygon type and index triples into vertex/normal/texcoord arrays.
// Polygon == 4 means quad (two triangles: 0-1-2 and 0-2-3).
type Triangle struct {
	Polygon int
	VI      [4]int16
	NI      [4]int16
	TI      [4]int16
}

// Mesh holds parsed geometry for one sub-mesh within a BMD file.
type Mesh struct {
	Verts   [][3]float32 // vertex positions, mutable for bone transforms
	Nodes   []int16      // bone index per vertex
	Normals [][3]float32
	UVs     [][2]float32
	Tris    []Triangle
	TexPath string // texture reference from BMD (e.g. "sword04.jpg")
}

// Bone holds bind-pose data for one bone in the skeleton hierarchy.
type Bone struct {
	Parent       int
	IsDummy      bool
	BindPosition [3]float64
	BindRotation [3]float64 // Euler XYZ radians
}
