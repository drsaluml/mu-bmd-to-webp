package trs

// Entry holds per-item transform data from ItemTRSData.bmd + custom overrides.
type Entry struct {
	PosX, PosY, PosZ float64
	RotX, RotY, RotZ float64
	Scale            float64
	Source           string // "binary" or "custom"

	// Optional overrides from custom_trs.json
	UseBones     *bool   // nil = auto, true/false = forced
	Standardize  *bool   // nil = true (default), false = skip PCA rotation
	DisplayAngle float64 // PCA target angle in degrees (default -45)
	FillRatio    float64 // canvas fill fraction (default 0.70)
	Flip         bool    // invert auto-orientation detection
	Camera       string  // "", "noflip", "correction", "fallback"
	Perspective    bool    // enable perspective projection
	FOV            float64 // field of view in degrees (default 75)
	KeepAllMeshes  bool    // skip effect mesh filtering
}

// Data maps (section, index) to an Entry.
type Data map[[2]int]*Entry

// DefaultDisplayAngle is the default PCA target angle.
const DefaultDisplayAngle = -45.0

// DefaultFillRatio is the default canvas fill fraction.
const DefaultFillRatio = 0.70

// DefaultFOV is the default field of view.
const DefaultFOV = 75.0
