# MU Online BMD 3D Renderer

A CLI tool for converting MU Online's BMD (Binary Model Data) 3D model files
into transparent WebP images (default 256x256, supports rectangular output) for web display.

Written in Pure Go (no CGo) — supports cross-compilation.

[ภาษาไทย (Thai)](README_TH.md)

## Features

- Decrypts all 4 BMD versions: v10 (unencrypted), v12 (XOR), v14 (ModulusCryptor), v15 (LEA-256 ECB)
- Loads OZJ (JPEG) and OZT (TGA) textures with concurrent caching
- Software rasterizer: z-buffer, barycentric UV interpolation, bilinear texture sampling
- 4-pass blending: opaque, alpha blend, additive, force-additive under-composite
- Triple light source (main, rim, ambient) with ACES Filmic tone mapping
- Bone skinning (rigid, 1 bone per vertex)
- Automatic effect mesh filtering (glow/aura/gradient effects)
- Body mesh filtering (removes character body/skin/hair from equipment models)
- 3-tier camera system: Correction, Fallback, Noflip (auto-selected based on TRS)
- 2x supersampling with Lanczos downsample
- PCA rotation alignment + auto scale-to-fill
- Mirror pair mode for symmetric items (gloves, boots)
- Color tint system for per-item/per-mesh colorization
- Perspective projection mode with positioned camera
- Parallel processing (goroutine worker pool)
- Lossless WebP output (VP8L)
- Item list decoder: converts encrypted `item.bmd` to `ItemList.xml`

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

### Using `go run` (no build needed)

```bash
cd mu-bmd-renderer

# Render all items (~22 sec, goroutine workers)
go run ./cmd/render -config config.json

# Render first N items for testing
go run ./cmd/render -config config.json -test 20

# Render a specific section
go run ./cmd/render -config config.json -section 0

# Render a single item (section 0, index 3 = Katana)
go run ./cmd/render -config config.json -section 0 -index 3

# Custom worker count
go run ./cmd/render -config config.json -workers 8
```

### Using binary (build first)

```bash
make build
./mu-bmd-renderer -config config.json
./mu-bmd-renderer -test 20
./mu-bmd-renderer -section 0 -index 3
```

### Decode item.bmd to ItemList.xml

Convert the encrypted `item.bmd` binary into the `ItemList.xml` format used by the renderer.
This is useful for updating the item list from the latest game client data.

```bash
# Default: Data/Local/item.bmd → Data/Xml/ItemList.xml
go run ./cmd/decodeitem

# Custom paths
go run ./cmd/decodeitem path/to/item.bmd path/to/output.xml
```

The decoder:
- Decrypts XOR encryption (3-byte repeating key)
- Decodes Windows-1252 encoded item names to UTF-8
- Outputs XML in the exact format expected by the renderer
- Preserves all item attributes (stats, class flags, resistances, trade flags, etc.)

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
  "render_width": 0,
  "render_height": 0,
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
| `render_size` | Output image size in pixels (square shorthand, sets both width and height) |
| `render_width` | Output image width in pixels (0 = use `render_size`) |
| `render_height` | Output image height in pixels (0 = use `render_size`) |
| `supersample` | Supersampling multiplier (2 = render at 2x then downscale) |
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
│   │   ├── item.bmd         # Encrypted item list (source for decodeitem)
│   │   └── itemtrsdata.bmd  # Per-item rotation/scale data
│   ├── Xml/
│   │   └── ItemList.xml   # Complete item list (generated by decodeitem)
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
Supports presets, range syntax, section-wide and per-item overrides:

```json
{
  "presets": {
    "scroll": { "camera": "noflip", "rotX": 0, "rotY": 10, "fill_ratio": 0.60 }
  },
  "sections": {
    "6": { "rotZ": 90, "camera": "noflip", "bones": false, "override": true }
  },
  "items": {
    "1_4": { "rotX": -5, "rotY": 40, "rotZ": 40, "camera": "noflip" },
    "14_72-77": "scroll"
  }
}
```

### Presets

Define reusable TRS configurations in `"presets"`. Reference them by name as a string value in `"items"`:

```json
{
  "presets": {
    "scroll": { "camera": "noflip", "rotX": 0, "rotY": 10 }
  },
  "items": {
    "14_72-77": "scroll",
    "15_0": "scroll"
  }
}
```

### Range syntax

Use `{section}_{start}-{end}` to apply the same config to a range of items:

```json
{
  "items": {
    "10_0-19": "old_glove",
    "14_72-77": "scroll"
  }
}
```

### Override fields

| Field | Type | Description |
|-------|------|-------------|
| `rotX`, `rotY`, `rotZ` | float | Rotation angles (degrees) |
| `scale` | float | Model scale |
| `bones` | bool | Enable bone skinning |
| `display_angle` | float | Output image rotation angle (degrees, default: -45) |
| `fill_ratio` | float | Canvas fill ratio (0.0-1.0, default: 0.70) |
| `camera` | string | Camera mode: `"correction"`, `"noflip"`, `"fallback"` |
| `perspective` | bool | Use perspective projection |
| `fov` | float | Field of view for perspective (degrees, default: 75) |
| `cam_height` | float | Positioned camera height as fraction of model height (0 = disabled) |
| `flip` | bool | Invert blade orientation detection |
| `flip_canvas` | bool | Mirror final image horizontally |
| `override` | bool | (sections only) Force custom values over binary TRS for all items |
| `merge` | bool | (sections only) Merge specific fields into binary TRS |
| `standardize` | bool | Enable PCA rotation alignment (default: true) |
| `keep_all_meshes` | bool | Skip effect mesh filtering |
| `mirror_pair` | bool | Render one side then duplicate+mirror to create a pair |
| `additive_textures` | string[] | Force these texture stems to additive under-composite blending |
| `additive_on_top` | bool | Use additive on top (Pass 3b) instead of under-composite (Pass 4) for additive_textures |
| `additive_floor` | int | luminanceAlpha floor for force-additive pass (default: 40) |
| `exclude_textures` | string[] | Exclude meshes with these texture stems |
| `bone_flip` | bool | Prefix root bone matrices with Rx(-90°) for models with non-standard bone hierarchy |
| `tint` | [R,G,B] | RGB color multiplier (e.g. `[1, 0.3, 0.3]` for red tint) |
| `tint_textures` | string[] | Apply tint only to matching texture stems |
| `render_width` | int | Per-item output width override (0 = use global config) |
| `render_height` | int | Per-item output height override (0 = use global config) |

Item keys use the format `{section}_{index}`, e.g. `"1_4"` = section 1, index 4.

## Code Structure

```
mu-bmd-renderer/
├── cmd/
│   ├── render/main.go         # CLI entry point (renderer)
│   └── decodeitem/main.go     # item.bmd → ItemList.xml decoder
├── internal/
│   ├── config/                # Config loading and path resolution
│   ├── crypto/                # LEA-256 ECB, XOR, and ModulusCryptor decryption
│   ├── bmd/                   # BMD file parser → meshes + bones
│   ├── texture/               # OZJ/OZT loader + concurrent cache
│   ├── trs/                   # Rotation/scale data loader (binary + custom + presets)
│   ├── itemlist/              # ItemList.xml parser
│   ├── mathutil/              # Vec3, Mat3, Mat4, Quaternion, PCA
│   ├── skeleton/              # Bone world matrices + skinning
│   ├── filter/                # Effect mesh + body mesh + glow layer filters
│   ├── viewmatrix/            # View matrix computation (3 modes + positioned camera)
│   ├── raster/                # Software rasterizer (4-pass blending)
│   ├── postprocess/           # Cluster removal, PCA alignment, supersample, mirror pair
│   └── batch/                 # Worker pool + manifest.json
├── config.json                # Config file
├── config.example.json        # Config template
├── custom_trs.json            # Per-item camera angle overrides
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
| Total time (4812 items) | ~47 min | ~22 sec |
| Speed | ~1.7 items/sec | ~220 items/sec |
| Success rate | 98.1% | 99.5% |
| Processing | Sequential | Parallel (goroutine pool) |

Approximately **130x faster** than Python.

## Item Sections

| Section | Name | Items |
|---------|------|-------|
| 0 | Swords | 136 |
| 1 | Axes | 9 |
| 2 | Maces and Scepters | 72 |
| 3 | Spears | 36 |
| 4 | Bows and Crossbows | 74 |
| 5 | Staffs | 156 |
| 6 | Shields | 114 |
| 7 | Helmets | 375 |
| 8 | Armors | 411 |
| 9 | Pants | 411 |
| 10 | Gloves | 390 |
| 11 | Boots | 411 |
| 12 | Pets, Rings and Misc | 436 |
| 13 | Jewels and Misc | 470 |
| 14 | Wings, Orbs and Spheres | 462 |
| 15 | Scrolls | 72 |
| 16 | Muuns | 389 |
| 19 | Uncategorized | 39 |
| 20 | Uncategorized | 344 |
| 21 | Cloaks | 5 |
