# MU Online BMD 3D Renderer

โปรแกรม CLI สำหรับแปลงไฟล์โมเดล 3D BMD (Binary Model Data) จากเกม MU Online
ให้เป็นภาพ WebP ขนาด 256x256 พิกเซล พร้อมพื้นหลังโปร่งใส สำหรับใช้แสดงผลบนเว็บ

เขียนด้วย Go (Pure Go, ไม่ใช้ CGo) รองรับ cross-compilation

## ความสามารถ

- ถอดรหัส BMD ทั้ง 3 เวอร์ชัน: v10 (ไม่เข้ารหัส), v12 (XOR), v15 (LEA-256 ECB)
- โหลด texture OZJ (JPEG) และ OZT (TGA) พร้อม cache แบบ concurrent
- Software rasterizer: z-buffer, barycentric UV interpolation, bilinear texture sampling
- ระบบแสงแบบ 3 แหล่ง (main, rim, ambient) พร้อม ACES Filmic tone mapping
- Bone skinning (rigid, 1 bone ต่อ vertex)
- กรอง effect mesh (เอฟเฟกต์เรือง/ออร่า) ออกอัตโนมัติ
- ระบบมุมกล้อง 3 แบบ: Correction, Fallback, Noflip (เลือกอัตโนมัติตาม TRS)
- Supersampling 2x พร้อม Lanczos downsample
- PCA rotation alignment + auto scale-to-fill
- ประมวลผลแบบขนาน (goroutine worker pool)
- บันทึกเป็น WebP (lossless VP8L)

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
│   │   └── itemtrsdata.bmd  # ข้อมูลมุมหมุน/สเกลของแต่ละไอเทม
│   ├── Xml/
│   │   └── ItemList.xml   # รายการไอเทมทั้งหมด
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
รองรับการกำหนดค่าทั้งแบบ section (ทั้งหมวด) และแบบรายไอเทม:

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

| ฟิลด์ | คำอธิบาย |
|-------|----------|
| `rotX`, `rotY`, `rotZ` | มุมหมุน (องศา) |
| `scale` | สเกลโมเดล |
| `bones` | ใช้ bone skinning หรือไม่ (`true`/`false`) |
| `display_angle` | มุมหมุนภาพ output (องศา, ค่าเริ่มต้น: -45) |
| `fill_ratio` | สัดส่วนการเติมเต็มภาพ (0.0-1.0, ค่าเริ่มต้น: 0.70) |
| `camera` | โหมดกล้อง: `"correction"`, `"noflip"`, `"fallback"` |
| `perspective` | ใช้ perspective projection (`true`/`false`) |
| `fov` | field of view สำหรับ perspective (องศา, ค่าเริ่มต้น: 75) |
| `flip` | กลับภาพซ้าย-ขวา (`true`/`false`) |
| `override` | บังคับใช้ค่าจาก custom แทนค่าจาก binary TRS ทั้งหมด |

key ของ items ใช้รูปแบบ `{section}_{index}` เช่น `"1_4"` = section 1, index 4

## โครงสร้างโค้ด

```
mu-bmd-renderer/
├── cmd/render/main.go          # CLI entry point
├── internal/
│   ├── config/                 # โหลดและ resolve ค่า config
│   ├── crypto/                 # ถอดรหัส LEA-256 ECB, XOR
│   ├── bmd/                    # อ่านไฟล์ BMD → meshes + bones
│   ├── texture/                # โหลด OZJ/OZT + cache concurrent
│   ├── trs/                    # โหลดข้อมูลมุมหมุน/สเกล
│   ├── itemlist/               # อ่าน ItemList.xml
│   ├── mathutil/               # Vec3, Mat3, Mat4, Quaternion, PCA
│   ├── skeleton/               # Bone world matrices + skinning
│   ├── filter/                 # กรอง effect mesh + connected components
│   ├── viewmatrix/             # คำนวณ view matrix (3 โหมด)
│   ├── raster/                 # Software rasterizer (hot path)
│   ├── postprocess/            # ลบ cluster เล็ก, PCA alignment, supersample
│   └── batch/                  # Worker pool + manifest.json
├── config.json                 # ไฟล์ config
├── config.example.json         # ตัวอย่างไฟล์ config
├── custom_trs.json             # ปรับแต่งมุมกล้องรายไอเทม
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
| อัตราสำเร็จ | 98.1% | 98.1% |
| การประมวลผล | ลำดับเดียว | ขนาน (goroutine pool) |

เร็วกว่า Python ประมาณ **130 เท่า**
