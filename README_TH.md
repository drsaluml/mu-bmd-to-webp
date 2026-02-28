# MU Online BMD 3D Renderer

โปรแกรม CLI สำหรับแปลงไฟล์โมเดล 3D BMD (Binary Model Data) จากเกม MU Online
ให้เป็นภาพ WebP ขนาด 256x256 พิกเซล พร้อมพื้นหลังโปร่งใส สำหรับใช้แสดงผลบนเว็บ

เขียนด้วย Go (Pure Go, ไม่ใช้ CGo) รองรับ cross-compilation

[English](README.md)

## ความสามารถ

- ถอดรหัส BMD ทั้ง 4 เวอร์ชัน: v10 (ไม่เข้ารหัส), v12 (XOR), v14 (ModulusCryptor), v15 (LEA-256 ECB)
- โหลด texture OZJ (JPEG) และ OZT (TGA) พร้อม cache แบบ concurrent
- Software rasterizer: z-buffer, barycentric UV interpolation, bilinear texture sampling
- ระบบ blending 4 รอบ: opaque, alpha blend, additive, force-additive under-composite
- ระบบแสงแบบ 3 แหล่ง (main, rim, ambient) พร้อม ACES Filmic tone mapping
- Bone skinning (rigid, 1 bone ต่อ vertex)
- กรอง effect mesh (เอฟเฟกต์เรือง/ออร่า/gradient) ออกอัตโนมัติ
- กรอง body mesh (ตัวละคร/ผิวหนัง/ผม) ออกจากโมเดลอุปกรณ์อัตโนมัติ
- ระบบมุมกล้อง 3 แบบ: Correction, Fallback, Noflip (เลือกอัตโนมัติตาม TRS)
- Supersampling 2x พร้อม Lanczos downsample
- PCA rotation alignment + auto scale-to-fill
- โหมด mirror pair สำหรับไอเทมคู่สมมาตร (ถุงมือ, รองเท้า)
- ระบบ color tint สำหรับปรับสีรายไอเทม/ราย mesh
- โหมด perspective projection พร้อมกล้องแบบกำหนดตำแหน่ง
- ประมวลผลแบบขนาน (goroutine worker pool)
- บันทึกเป็น WebP (lossless VP8L)
- ตัวถอดรหัส item list: แปลง `item.bmd` เข้ารหัสเป็น `ItemList.xml`

## ความต้องการ

- Go 1.24+

## การติดตั้ง

```bash
cd mu-bmd-renderer

# ดาวน์โหลด dependencies
make deps

# build
make build
```

## การใช้งาน

### เรนเดอร์ทั้งหมด

```bash
./mu-bmd-renderer
```

### เรนเดอร์เพื่อทดสอบ (N ไอเทมแรก)

```bash
./mu-bmd-renderer -test 20
```

### เรนเดอร์เฉพาะ section

```bash
# ทุกไอเทมใน section 0 (Swords)
./mu-bmd-renderer -section 0

# เฉพาะ Katana (section 0, index 3)
./mu-bmd-renderer -section 0 -index 3
```

### ใช้ไฟล์ config

```bash
./mu-bmd-renderer -config config.json
```

### ถอดรหัส item.bmd เป็น ItemList.xml

แปลงไฟล์ `item.bmd` เข้ารหัสเป็นรูปแบบ `ItemList.xml` ที่ renderer ใช้งาน
สำหรับอัพเดทรายการไอเทมจากข้อมูลล่าสุดของ game client

```bash
# ค่าเริ่มต้น: Data/Local/item.bmd → Data/Xml/ItemList.xml
go run ./cmd/decodeitem

# กำหนด path เอง
go run ./cmd/decodeitem path/to/item.bmd path/to/output.xml
```

ตัวถอดรหัส:
- ถอดรหัส XOR encryption (3-byte repeating key)
- แปลงชื่อไอเทมจาก Windows-1252 เป็น UTF-8
- ส่งออก XML ตามรูปแบบที่ renderer ต้องการ
- รักษา attribute ทั้งหมด (สถิติ, class flags, ค่าต้านทาน, trade flags ฯลฯ)

### CLI flags ทั้งหมด

| Flag | ค่าเริ่มต้น | คำอธิบาย |
|------|------------|----------|
| `-config` | _(ไม่ใช้)_ | path ไปยังไฟล์ config.json |
| `-data` | _(auto-detect)_ | path ไปยัง base directory ที่มีโฟลเดอร์ `Data/` |
| `-output` | `Data/Item-renders` | โฟลเดอร์ output |
| `-test` | `0` | เรนเดอร์เฉพาะ N ไอเทมแรก |
| `-section` | `-1` | เรนเดอร์เฉพาะ section ที่กำหนด |
| `-index` | `-1` | เรนเดอร์เฉพาะ index ที่กำหนด (ต้องใช้คู่กับ `-section`) |
| `-workers` | จำนวน CPU | จำนวน goroutine สำหรับประมวลผลแบบขนาน |
| `-quality` | `90` | คุณภาพ WebP (1-100) |

## ไฟล์ config

สร้างไฟล์ `config.json` เพื่อกำหนด path และค่าเรนเดอร์ต่างๆ:

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

| ฟิลด์ | คำอธิบาย |
|-------|----------|
| `base_dir` | โฟลเดอร์หลักของโปรเจค (ว่าง = auto-detect) |
| `item_dir` | โฟลเดอร์ที่เก็บไฟล์ BMD |
| `item_list_xml` | path ไปยัง ItemList.xml |
| `trs_bmd` | path ไปยัง itemtrsdata.bmd (ข้อมูลมุมหมุน/สเกล) |
| `custom_trs_json` | path ไปยัง custom_trs.json (ปรับแต่งมุมเพิ่มเติม) |
| `output_dir` | โฟลเดอร์สำหรับเก็บภาพ output |
| `render_size` | ขนาดภาพ output (พิกเซล) |
| `supersample` | ตัวคูณ supersampling (2 = เรนเดอร์ 512 แล้วย่อเหลือ 256) |
| `webp_quality` | คุณภาพ WebP (1-100) |
| `workers` | จำนวน worker (0 = ใช้ทุก CPU) |

path ที่เป็น relative จะถูก resolve ตาม `base_dir`

ลำดับความสำคัญ: **CLI flags > config.json > auto-detect**

## โครงสร้างโฟลเดอร์ที่ต้องมี

```
base_dir/
├── Data/
│   ├── Item/              # ไฟล์ BMD โมเดล 3D
│   │   └── texture/       # ไฟล์ texture (*.ozj, *.ozt)
│   ├── Local/
│   │   ├── item.bmd         # รายการไอเทมเข้ารหัส (ต้นทางสำหรับ decodeitem)
│   │   └── itemtrsdata.bmd  # ข้อมูลมุมหมุน/สเกลของแต่ละไอเทม
│   ├── Xml/
│   │   └── ItemList.xml   # รายการไอเทมทั้งหมด (สร้างโดย decodeitem)
│   └── Item-renders/      # ← ภาพ output จะถูกสร้างที่นี่
└── custom_trs.json        # (ไม่บังคับ) ปรับแต่งมุมหมุนเพิ่มเติม
```

## Output

ภาพจะถูกบันทึกตามโครงสร้าง:

```
Data/Item-renders/
├── 0/
│   ├── 0.webp    # Kris
│   ├── 1.webp    # Short Sword
│   ├── 2.webp    # Rapier
│   └── ...
├── 1/
│   └── ...
├── manifest.json  # รายการไอเทมทั้งหมดที่เรนเดอร์แล้ว
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

ไฟล์สำหรับปรับแต่งมุมกล้องของไอเทมที่เรนเดอร์ออกมาไม่สวย
รองรับ presets, range syntax, การกำหนดค่าทั้งแบบ section และแบบรายไอเทม:

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

กำหนดค่า TRS ที่ใช้ซ้ำได้ใน `"presets"` แล้วอ้างอิงด้วยชื่อเป็น string ใน `"items"`:

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

ใช้ `{section}_{start}-{end}` เพื่อกำหนดค่าเดียวกันให้ไอเทมหลายตัว:

```json
{
  "items": {
    "10_0-19": "old_glove",
    "14_72-77": "scroll"
  }
}
```

### ฟิลด์ที่ปรับได้

| ฟิลด์ | ชนิด | คำอธิบาย |
|-------|------|----------|
| `rotX`, `rotY`, `rotZ` | float | มุมหมุน (องศา) |
| `scale` | float | สเกลโมเดล |
| `bones` | bool | ใช้ bone skinning หรือไม่ |
| `display_angle` | float | มุมหมุนภาพ output (องศา, ค่าเริ่มต้น: -45) |
| `fill_ratio` | float | สัดส่วนการเติมเต็มภาพ (0.0-1.0, ค่าเริ่มต้น: 0.70) |
| `camera` | string | โหมดกล้อง: `"correction"`, `"noflip"`, `"fallback"` |
| `perspective` | bool | ใช้ perspective projection |
| `fov` | float | field of view สำหรับ perspective (องศา, ค่าเริ่มต้น: 75) |
| `cam_height` | float | ตำแหน่งกล้องเป็นสัดส่วนของความสูงโมเดล (0 = ปิด) |
| `flip` | bool | กลับทิศใบดาบ |
| `flip_canvas` | bool | กลับภาพซ้าย-ขวา |
| `override` | bool | (sections เท่านั้น) บังคับใช้ค่า custom แทน binary TRS ทั้งหมด |
| `merge` | bool | (sections เท่านั้น) ผสานฟิลด์เฉพาะเข้ากับ binary TRS |
| `standardize` | bool | เปิด PCA rotation alignment (ค่าเริ่มต้น: true) |
| `keep_all_meshes` | bool | ข้ามการกรอง effect mesh |
| `mirror_pair` | bool | เรนเดอร์ข้างเดียวแล้ว duplicate+mirror สร้างคู่ |
| `additive_textures` | string[] | บังคับ texture stems เหล่านี้เป็น additive under-composite |
| `exclude_textures` | string[] | ไม่เรนเดอร์ mesh ที่มี texture stems เหล่านี้ |
| `tint` | [R,G,B] | ตัวคูณสี RGB (เช่น `[1, 0.3, 0.3]` = โทนแดง) |
| `tint_textures` | string[] | ใช้ tint เฉพาะ texture stems ที่ตรงกัน |

key ของ items ใช้รูปแบบ `{section}_{index}` เช่น `"1_4"` = section 1, index 4

## โครงสร้างโค้ด

```
mu-bmd-renderer/
├── cmd/
│   ├── render/main.go         # CLI entry point (renderer)
│   └── decodeitem/main.go     # ตัวถอดรหัส item.bmd → ItemList.xml
├── internal/
│   ├── config/                # โหลดและ resolve ค่า config
│   ├── crypto/                # ถอดรหัส LEA-256 ECB, XOR, ModulusCryptor
│   ├── bmd/                   # อ่านไฟล์ BMD → meshes + bones
│   ├── texture/               # โหลด OZJ/OZT + cache concurrent
│   ├── trs/                   # โหลดข้อมูลมุมหมุน/สเกล (binary + custom + presets)
│   ├── itemlist/              # อ่าน ItemList.xml
│   ├── mathutil/              # Vec3, Mat3, Mat4, Quaternion, PCA
│   ├── skeleton/              # Bone world matrices + skinning
│   ├── filter/                # กรอง effect mesh + body mesh + glow layers
│   ├── viewmatrix/            # คำนวณ view matrix (3 โหมด + positioned camera)
│   ├── raster/                # Software rasterizer (blending 4 รอบ)
│   ├── postprocess/           # ลบ cluster เล็ก, PCA alignment, supersample, mirror pair
│   └── batch/                 # Worker pool + manifest.json
├── config.json                # ไฟล์ config
├── config.example.json        # ตัวอย่างไฟล์ config
├── custom_trs.json            # ปรับแต่งมุมกล้องรายไอเทม
├── Makefile
├── go.mod
└── go.sum
```

## Makefile

```bash
make build        # build binary
make run          # build + run ทั้งหมด
make test         # run unit tests
make test-quick   # build + render 5 ไอเทมแรก
make test-single  # build + render Katana (section 0, index 3)
make lint         # go vet
make clean        # ลบ binary
make tidy         # go mod tidy
make deps         # go mod download
```

## ประสิทธิภาพ

| | Python | Go |
|---|---|---|
| เวลาทั้งหมด (4800 items) | ~47 นาที | ~22 วินาที |
| ความเร็ว | ~1.7 items/sec | ~220 items/sec |
| อัตราสำเร็จ | 98.1% | 99.4% |
| การประมวลผล | ลำดับเดียว | ขนาน (goroutine pool) |

เร็วกว่า Python ประมาณ **130 เท่า**

## รายชื่อ Section

| Section | ชื่อ | จำนวนไอเทม |
|---------|------|-----------|
| 0 | Swords (ดาบ) | 136 |
| 1 | Axes (ขวาน) | 9 |
| 2 | Maces and Scepters (กระบอง/คทา) | 72 |
| 3 | Spears (หอก) | 36 |
| 4 | Bows and Crossbows (ธนู/หน้าไม้) | 74 |
| 5 | Staffs (ไม้เท้า) | 156 |
| 6 | Shields (โล่) | 114 |
| 7 | Helmets (หมวก) | 375 |
| 8 | Armors (เกราะ) | 411 |
| 9 | Pants (กางเกง) | 411 |
| 10 | Gloves (ถุงมือ) | 390 |
| 11 | Boots (รองเท้า) | 411 |
| 12 | Pets, Rings and Misc (สัตว์เลี้ยง/แหวน/อื่นๆ) | 436 |
| 13 | Jewels and Misc (อัญมณี/อื่นๆ) | 470 |
| 14 | Wings, Orbs and Spheres (ปีก/ออร์บ/สเฟียร์) | 462 |
| 15 | Scrolls (ม้วนกระดาษ) | 72 |
| 16 | Muuns (มูน) | 389 |
| 19 | Uncategorized (ไม่จัดหมวด) | 39 |
| 20 | Uncategorized (ไม่จัดหมวด) | 344 |
| 21 | Cloaks (เสื้อคลุม) | 5 |
