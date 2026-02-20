# MU Online BMD 3D Renderer

A CLI tool for converting MU Online's BMD (Binary Model Data) 3D model files
into transparent 256x256 WebP images for web display.

Written in Pure Go (no CGo) — supports cross-compilation.

[ภาษาไทย (Thai)](README_TH.md)

## Features

- Decrypts all 3 BMD versions: v10 (unencrypted), v12 (XOR), v15 (LEA-256 ECB)
- Loads OZJ (JPEG) and OZT (TGA) textures with concurrent caching
- Software rasterizer: z-buffer, barycentric UV interpolation, bilinear texture sampling
- Triple light source (main, rim, ambient) with ACES Filmic tone mapping
- Bone skinning (rigid, 1 bone per vertex)
- Automatic effect mesh filtering (glow/aura effects)
- 3-tier camera system: Correction, Fallback, Noflip (auto-selected based on TRS)
- 2x supersampling with Lanczos downsample
- PCA rotation alignment + auto scale-to-fill
- Parallel processing (goroutine worker pool)
- Lossless WebP output (VP8L)

## Requirements

- Go 1.24+

## Installation

```bash
cd mu-bmd-renderer

# Download dependencies
make deps

# Build
make build
```

## Usage

### Render all items

```bash
./mu-bmd-renderer
```

### Render for testing (first N items)

```bash
./mu-bmd-renderer -test 20
```

### Render a specific section

```bash
# All items in section 0 (Swords)
./mu-bmd-renderer -section 0

# Only Katana (section 0, index 3)
./mu-bmd-renderer -section 0 -index 3
```

### Use a config file

```bash
./mu-bmd-renderer -config config.json
```

### All CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | _(none)_ | Path to config.json file |
| `-data` | _(auto-detect)_ | Path to base directory containing `Data/` |
| `-output` | `Data/Item-renders` | Output directory |
| `-test` | `0` | Render only the first N items |
| `-section` | `-1` | Render only the specified section |
| `-index` | `-1` | Render only the specified index (requires `-section`) |
| `-workers` | CPU count | Number of goroutines for parallel processing |
| `-quality` | `90` | WebP quality (1-100) |

## Config File

Create a `config.json` file to set paths and render options:

```json
{
  "base_dir": "",
  "item_dir": "Data/Item",
  "item_list_xml": "Data/Xml/ItemList.xml",
  "trs_bmd": "Data/Local/itemtrsdata.bmd",
  "custom_trs_json": "custom_trs.json",
  "output_dir": "Data/Item-renders",
  "render_size": 256,
  "supersample": 2,
  "webp_quality": 90,
  "workers": 0
}
```

| Field | Description |
|-------|-------------|
| `base_dir` | Project base directory (empty = auto-detect) |
| `item_dir` | Directory containing BMD files |
| `item_list_xml` | Path to ItemList.xml |
| `trs_bmd` | Path to itemtrsdata.bmd (rotation/scale data) |
| `custom_trs_json` | Path to custom_trs.json (custom angle overrides) |
| `output_dir` | Output directory for rendered images |
| `render_size` | Output image size (pixels) |
| `supersample` | Supersampling multiplier (2 = render at 512 then downscale to 256) |
| `webp_quality` | WebP quality (1-100) |
| `workers` | Number of workers (0 = use all CPUs) |

Relative paths are resolved against `base_dir`.

Priority order: **CLI flags > config.json > auto-detect**

## Required Directory Structure

```
base_dir/
├── Data/
│   ├── Item/              # BMD 3D model files
│   │   └── texture/       # Texture files (*.ozj, *.ozt)
│   ├── Local/
│   │   └── itemtrsdata.bmd  # Per-item rotation/scale data
│   ├── Xml/
│   │   └── ItemList.xml   # Complete item list
│   └── Item-renders/      # ← Output images are generated here
└── custom_trs.json        # (Optional) Custom angle overrides
```

## Output

Images are saved in the following structure:

```
Data/Item-renders/
├── 0/
│   ├── 0.webp    # Kris
│   ├── 1.webp    # Short Sword
│   ├── 2.webp    # Rapier
│   └── ...
├── 1/
│   └── ...
├── manifest.json  # List of all rendered items
└── ...
```

### manifest.json

```json
[
  {
    "section": 0,
    "section_name": "Swords",
    "index": 3,
    "name": "Katana",
    "model_file": "Sword04.bmd",
    "image": "0/3.webp"
  }
]
```

## custom_trs.json

A file for adjusting camera angles of items that don't render well by default.
Supports both section-wide and per-item overrides:

```json
{
  "sections": {
    "1": { "rotX": 0, "rotY": 18, "rotZ": 0, "scale": 0.0025, "bones": false }
  },
  "items": {
    "1_4": { "rotX": -5, "rotY": 40, "rotZ": 40, "camera": "noflip", "perspective": true, "fov": 45 }
  }
}
```

| Field | Description |
|-------|-------------|
| `rotX`, `rotY`, `rotZ` | Rotation angles (degrees) |
| `scale` | Model scale |
| `bones` | Enable bone skinning (`true`/`false`) |
| `display_angle` | Output image rotation angle (degrees, default: -45) |
| `fill_ratio` | Canvas fill ratio (0.0-1.0, default: 0.70) |
| `camera` | Camera mode: `"correction"`, `"noflip"`, `"fallback"` |
| `perspective` | Use perspective projection (`true`/`false`) |
| `fov` | Field of view for perspective (degrees, default: 75) |
| `flip` | Flip image horizontally (`true`/`false`) |
| `override` | Force custom values over binary TRS for all items in section |

Item keys use the format `{section}_{index}`, e.g. `"1_4"` = section 1, index 4.

## Code Structure

```
mu-bmd-renderer/
├── cmd/render/main.go          # CLI entry point
├── internal/
│   ├── config/                 # Config loading and path resolution
│   ├── crypto/                 # LEA-256 ECB and XOR decryption
│   ├── bmd/                    # BMD file parser → meshes + bones
│   ├── texture/                # OZJ/OZT loader + concurrent cache
│   ├── trs/                    # Rotation/scale data loader
│   ├── itemlist/               # ItemList.xml parser
│   ├── mathutil/               # Vec3, Mat3, Mat4, Quaternion, PCA
│   ├── skeleton/               # Bone world matrices + skinning
│   ├── filter/                 # Effect mesh filter + connected components
│   ├── viewmatrix/             # View matrix computation (3 modes)
│   ├── raster/                 # Software rasterizer (hot path)
│   ├── postprocess/            # Cluster removal, PCA alignment, supersample
│   └── batch/                  # Worker pool + manifest.json
├── config.json                 # Config file
├── config.example.json         # Config template
├── custom_trs.json             # Per-item camera angle overrides
├── Makefile
├── go.mod
└── go.sum
```

## Makefile

```bash
make build        # Build binary
make run          # Build + render all items
make test         # Run unit tests
make test-quick   # Build + render first 5 items
make test-single  # Build + render Katana (section 0, index 3)
make lint         # Run go vet
make clean        # Remove binary
make tidy         # Run go mod tidy
make deps         # Download dependencies
```

## Performance

| | Python | Go |
|---|---|---|
| Total time (4800 items) | ~47 min | ~22 sec |
| Speed | ~1.7 items/sec | ~220 items/sec |
| Success rate | 98.1% | 98.1% |
| Processing | Sequential | Parallel (goroutine pool) |

Approximately **130x faster** than Python.
