# คู่มือ custom_trs.json (ภาษาไทย)

คู่มืออธิบายตัวแปรทั้งหมดใน `custom_trs.json` พร้อมตัวอย่างการใช้งานจริง
สำหรับโปรเจกต์ MU Online BMD → WebP Renderer

---

## สารบัญ

1. [โครงสร้างไฟล์](#1-โครงสร้างไฟล์)
2. [ลำดับความสำคัญ](#2-ลำดับความสำคัญ-priority)
3. [ตัวแปรทั้งหมด](#3-ตัวแปรทั้งหมด)
4. [ส่วน presets](#4-ส่วน-presets)
5. [ส่วน sections](#5-ส่วน-sections)
6. [ส่วน models](#6-ส่วน-models)
7. [ส่วน items](#7-ส่วน-items)
8. [ตัวอย่างสถานการณ์จริง](#8-ตัวอย่างสถานการณ์จริง)

---

## 1. โครงสร้างไฟล์

ไฟล์ `custom_trs.json` มี 4 ส่วนหลัก:

```json
{
  "presets":  { ... },
  "sections": { ... },
  "models":   { ... },
  "items":    { ... }
}
```

| ส่วน | หน้าที่ |
|------|--------|
| `presets` | ประกาศค่าตั้งต้นที่ใช้ซ้ำได้ (เหมือนตัวแปรกลาง) |
| `sections` | กำหนดค่าทั้ง section (เช่น section 0 = ดาบทุกเล่ม) |
| `models` | กำหนดค่าตาม BMD model file (ข้าม section ได้) |
| `items` | กำหนดค่าเฉพาะ item (ละเอียดที่สุด, ชนะทุกอย่าง) |

---

## 2. ลำดับความสำคัญ (Priority)

```
items  >  models  >  sections  >  binary TRS (itemtrsdata.bmd)
สูงสุด                                              ต่ำสุด
```

- ถ้า item มีค่าใน `items` → ใช้ค่านั้น (ไม่สนค่าจาก models/sections/binary)
- ถ้าไม่มีใน `items` แต่มีใน `models` → ใช้ค่าจาก models
- ถ้าไม่มีใน `models` แต่ section มี config → ขึ้นกับ mode (override/merge/default)
- ถ้าไม่มีอะไรเลย → ใช้ binary TRS จาก `itemtrsdata.bmd`
- ถ้าไม่มี binary TRS ด้วย → ใช้ค่า default ของระบบ

**สำคัญ**: `items` **แทนที่ทั้งหมด** ไม่ merge — ต้องใส่ทุกค่าที่ต้องการ (เช่น `camera`, `bones`)

---

## 3. ตัวแปรทั้งหมด

### 3.1 ตัวแปรหมุนโมเดล 3 มิติ (Rotation)

| ตัวแปร | ชนิด | ค่าเริ่มต้น | คำอธิบาย |
|--------|------|------------|----------|
| `rotX` | float | 0 | หมุนรอบแกน X (องศา) — เอียงหน้า/หลัง |
| `rotY` | float | 0 | หมุนรอบแกน Y (องศา) — หมุนซ้าย/ขวา |
| `rotZ` | float | 0 | หมุนรอบแกน Z (องศา) — เอียงเฉียง |

ลำดับการหมุน: **Rz @ Ry @ Rx** (extrinsic ZYX) ตาม MU Online

```
rotX = -90  → โมเดลหงายขึ้น (เห็นด้านบน)
rotX = 270  → เหมือน -90 (270 - 360 = -90)
rotX = 180  → กลับหัว (พลิกทิศใบมีด)
rotY = 270  → หมุนขวา 270° (เข้า CORRECTION camera)
rotY = 90   → หมุนขวา 90° (เข้า FALLBACK camera)
rotY = 0    → ไม่หมุน (เข้า NOFLIP camera)
```

**ตัวอย่าง**: ดาบมาตรฐาน
```json
{ "rotX": 180, "rotY": 270, "rotZ": 15 }
```
- `rotX: 180` = พลิกใบมีดให้ชี้ขึ้น
- `rotY: 270` = หมุนเข้ามุม CORRECTION camera
- `rotZ: 15` = เอียงเฉียงเล็กน้อย

**ตัวอย่าง**: ปีก
```json
{ "rotX": 270, "rotY": -10, "rotZ": 0 }
```
- `rotX: 270` = ปีกกางแนวนอน
- `rotY: -10` = หมุนเล็กน้อยให้เห็นมิติ

---

### 3.2 ขนาดโมเดล (Scale)

| ตัวแปร | ชนิด | ค่าเริ่มต้น | คำอธิบาย |
|--------|------|------------|----------|
| `scale` | float | 0.0025 | ตัวคูณขนาดโมเดล |

```
scale = 0.0025  → ขนาดปกติ (ค่ามาตรฐาน)
scale = 0.0035  → ใหญ่ขึ้น 40%
scale = 0.0055  → ใหญ่ขึ้น 120% (ใช้กับดาบ section 0)
scale = 0.002   → เล็กลง 20% (ใช้กับเสื้อคลุม section 21)
```

**ตัวอย่าง**: เสื้อคลุม (model ใหญ่ ต้องย่อ)
```json
{ "rotX": 270, "rotY": -10, "scale": 0.002, "display_angle": -90 }
```

---

### 3.3 กล้อง (Camera)

| ตัวแปร | ชนิด | ค่าที่รับ | คำอธิบาย |
|--------|------|----------|----------|
| `camera` | string | `"noflip"`, `"correction"`, `"fallback"` | บังคับใช้กล้องแบบระบุ |

**3 โหมดกล้อง**:

| กล้อง | สูตร | เหมาะกับ | ลักษณะ |
|-------|------|---------|--------|
| `correction` | TRS_CORRECTION @ trs_rot | อาวุธ (rotY~270) | ปรับมุมตาม TRS ของอาวุธ |
| `fallback` | VIEW_FALLBACK (มุมคงที่) | items ทั่วไป, muun, pets | กล้องตายตัว ไม่สนค่า rot |
| `noflip` | NOFLIP_CAM @ trs_rot | ปีก, เกราะ, กล่อง | ไม่พลิกโมเดล ควบคุม rotation เอง |

ถ้าไม่ระบุ `camera` → ระบบเลือกอัตโนมัติจาก `rotY`:
- `rotY` ห่างจาก 270° ไม่เกิน 45° → CORRECTION
- `rotY` ห่างจาก 90° ไม่เกิน 45° → FALLBACK
- อื่นๆ → NOFLIP

**ตัวอย่าง**: กล่องไอเทม (ต้องการควบคุมมุมเอง)
```json
{ "rotX": -75, "rotY": 15, "rotZ": 3.5, "camera": "noflip", "standardize": false }
```
- `noflip` = ให้ rot ทำงานตรงๆ ไม่มี MODEL_FLIP
- ถ้าใช้ `fallback` → กล้องตายตัว rot ไม่มีผล

**ตัวอย่าง**: Muun pets (ใช้กล้องตายตัว)
```json
{ "camera": "fallback", "standardize": false }
```

---

### 3.4 กระดูก (Bones)

| ตัวแปร | ชนิด | ค่าเริ่มต้น | คำอธิบาย |
|--------|------|------------|----------|
| `bones` | bool | อัตโนมัติ | บังคับเปิด/ปิด bone transforms |

**พฤติกรรมอัตโนมัติ** (เมื่อไม่ระบุ):
- binary TRS → `false` (ข้ามกระดูก เพราะ TRS ปรับแล้ว)
- custom TRS → `true` (ใช้กระดูกเพื่อประกอบ mesh)
- ไม่มี TRS → `true`

**เมื่อไหร่ใช้ `bones: true`**:
- **รองเท้า/กางเกง** — mesh ไม่ประกอบโดยไม่มี bones (ขาแยกจากกัน)
- **ปีก/เสื้อคลุม** — bones ทำให้ mesh กางออก (ไม่มี bones = เส้นบาง)

**เมื่อไหร่ใช้ `bones: false`**:
- **ถุงมือ** — bones บีบ mesh เป็นท่าสวม (เล็กลง)
- **อาวุธ** — bones หมุนด้ามจับเป็นท่าถือ (ผิด)

**ตัวอย่าง**: รองเท้า (ต้องประกอบ mesh)
```json
{ "camera": "fallback", "display_angle": -90, "bones": true, "override": true }
```

**ตัวอย่าง**: ดาบ (ห้ามใช้ bones)
```json
{ "rotX": -45, "rotY": 40, "bones": false, "flip": true }
```

**คำเตือน**: per-item override **ไม่ inherit** `bones` จาก section — ต้องใส่เองทุกครั้ง!

---

### 3.5 การจัดภาพ PCA (Standardize / Display Angle)

| ตัวแปร | ชนิด | ค่าเริ่มต้น | คำอธิบาย |
|--------|------|------------|----------|
| `standardize` | bool | true | เปิด/ปิดการจัดภาพอัตโนมัติด้วย PCA |
| `display_angle` | float | -45 | มุมเป้าหมาย PCA (องศา จากแนวนอน) |
| `fill_ratio` | float | 0.70 | สัดส่วนเติมเต็มผ้าใบ (0.0 - 1.0) |
| `flip` | bool | false | สลับทิศ PCA (ปลาย ↔ ด้าม) |

#### standardize
เปิด/ปิดระบบจัดภาพอัตโนมัติ PCA — หมุนภาพ 2D ให้ item อยู่ในมุมสม่ำเสมอ

```
standardize = true  → PCA จัดมุม + ขยาย/ย่อให้พอดีผ้าใบ (default)
standardize = false → ใช้ภาพจากกล้องตรงๆ ไม่หมุน
```

**ใช้ `false` เมื่อ**: กล้องตายตัว (fallback) ดูดีอยู่แล้ว เช่น muuns, jewels, misc items

**ตัวอย่าง**:
```json
{ "camera": "fallback", "standardize": false }
```

#### display_angle
มุมเป้าหมายของ PCA — กำหนดว่า item เอียงกี่องศาในภาพสุดท้าย

```
display_angle = -90  → ตั้งตรง (แนวตั้ง)        ← รองเท้า, ปีก
display_angle = -60  → เอียงเล็กน้อย (~30° จากตั้งตรง)
display_angle = -45  → เอียง 45° (default)       ← อาวุธ
display_angle = -30  → เอียงมาก (~60° จากตั้งตรง) ← ดาบสไตล์ใหม่
display_angle = 0    → แนวนอน
```

**สูตรแปลง**: ต้องการเอียง N° จากแนวตั้ง → `display_angle = -(90 - N)`

**ตัวอย่าง**: ดาบเอียง 60° จากแนวตั้ง
```json
{ "display_angle": -30, "fill_ratio": 0.90 }
```

**ตัวอย่าง**: รองเท้าตั้งตรง
```json
{ "display_angle": -90 }
```

#### fill_ratio
สัดส่วนที่ item กินพื้นที่ผ้าใบ 256x256

```
fill_ratio = 0.50  → item กิน 50% ของผ้าใบ (เล็ก)
fill_ratio = 0.70  → 70% (default ค่าปกติ)
fill_ratio = 0.90  → 90% (ใหญ่เกือบเต็มผ้าใบ)
```

**ตัวอย่าง**: ดาบเล็กมาก ต้องขยายเต็มจอ
```json
{ "fill_ratio": 0.90, "display_angle": -60 }
```

**ตัวอย่าง**: item ที่มี detail เล็กรอบๆ ต้องเว้นขอบ
```json
{ "fill_ratio": 0.50 }
```

#### flip
สลับทิศทาง PCA — ปลายดาบ ↔ ด้ามจับ

```
flip = false → PCA เลือกทิศปกติ (default)
flip = true  → กลับทิศ (ปลายดาบสลับกับด้าม)
```

**ใช้เมื่อ**: PCA เลือกทิศผิด (ปลายดาบอยู่ล่างแทนที่จะอยู่บน)

**ตัวอย่าง**: ดาบที่ปลายชี้ผิดทาง
```json
{ "rotX": -45, "rotY": 40, "bones": false, "flip": true }
```

---

### 3.6 การพลิกภาพ (Flip Canvas)

| ตัวแปร | ชนิด | ค่าเริ่มต้น | คำอธิบาย |
|--------|------|------------|----------|
| `flip_canvas` | bool | false | พลิกภาพซ้าย-ขวา (mirror horizontal) หลัง post-process |

**ใช้เมื่อ**: render ออกมากลับด้านกับภาพอ้างอิง (BMD-viewer)

**ตัวอย่าง**: เครื่องรางที่ render กลับด้าน
```json
{ "camera": "fallback", "standardize": false, "flip_canvas": true }
```

---

### 3.7 การฉาย (Projection)

| ตัวแปร | ชนิด | ค่าเริ่มต้น | คำอธิบาย |
|--------|------|------------|----------|
| `perspective` | bool | false | เปิด perspective projection (มุมมอง 3 มิติ) |
| `fov` | float | 75 | มุมมองกว้าง (องศา) — ใช้กับ perspective เท่านั้น |

**ปกติ**: ระบบใช้ orthographic (ภาพแบน ไม่มีระยะใกล้/ไกล)
**เปิด perspective เมื่อ**: โมเดลแบน low-poly ที่ orthographic ทำให้เป็นเส้นตรง

```
fov = 75  → มุมกว้างปกติ (default)
fov = 45  → ซูมเข้า (เหมาะกับ item เล็ก)
fov = 90  → มุมกว้างมาก
```

**ตัวอย่าง**: ขวานแบน (Crescent Axe)
```json
{ "rotX": -5, "rotY": 40, "rotZ": 40, "camera": "noflip", "perspective": true, "fov": 45 }
```

---

### 3.8 เก็บ mesh ทั้งหมด (Keep All Meshes)

| ตัวแปร | ชนิด | ค่าเริ่มต้น | คำอธิบาย |
|--------|------|------------|----------|
| `keep_all_meshes` | bool | false | ข้าม effect mesh filter (เก็บ glow/aura/gradient) |

**ปกติ**: ระบบกรอง mesh ที่เป็น effect ออก (ออร่า, เรืองแสง, gradient)
**เปิดเมื่อ**: item ที่ mesh หลักถูกกรองผิด (หายไป)

**ตัวอย่าง**: Guardian Angel (mesh หลักถูกกรองเป็น effect)
```json
{ "camera": "fallback", "standardize": false, "keep_all_meshes": true }
```

---

### 3.9 ตัวแปรเฉพาะ section (Section-Only)

| ตัวแปร | ชนิด | ค่าเริ่มต้น | คำอธิบาย |
|--------|------|------------|----------|
| `override` | bool | false | แทนที่ binary TRS ทั้งหมดในหัวข้อนี้ |
| `merge` | bool | false | merge เฉพาะค่าที่ระบุเข้า binary TRS ที่มีอยู่ |

**3 โหมดของ section**:

| โหมด | พฤติกรรม | ใช้เมื่อ |
|------|---------|---------|
| **default** (ไม่มี override/merge) | ใส่ค่าเฉพาะ item ที่ **ไม่มี** binary TRS | ต้องการเพิ่มค่าให้ item ที่ขาด |
| **override = true** | **แทนที่** binary TRS ทุก item ใน section | ต้องการเปลี่ยนทั้ง section เหมือนกัน |
| **merge = true** | **merge** เฉพาะค่าที่ระบุเข้า binary TRS | ต้องการเพิ่มบาง field โดยไม่ทับ rotation |

**ตัวอย่าง override**: section 11 (รองเท้า) — แทนที่ binary TRS ทั้งหมด
```json
"11": { "camera": "fallback", "display_angle": -90, "bones": true, "override": true }
```
→ item ทุกตัวใน sec 11 ใช้ fallback camera + ตั้งตรง + ใช้ bones (ไม่ว่าจะมี binary TRS หรือไม่)

**ตัวอย่าง merge**: section 12 (ปีก/pets) — เพิ่ม display_angle + bones เข้า binary TRS
```json
"12": { "display_angle": -90, "bones": true, "merge": true }
```
→ item ที่มี binary TRS: เก็บ rotX/rotY/rotZ/scale เดิม แต่เพิ่ม display_angle=-90 + bones=true
→ item ที่ไม่มี binary TRS: สร้างใหม่ด้วยค่าจาก section config

**ตัวอย่าง default**: section 19 — ใส่ค่าเฉพาะ item ที่ไม่มี TRS
```json
"19": { "camera": "fallback", "standardize": false }
```
→ item ที่มี binary TRS: **ไม่เปลี่ยนแปลง** (binary TRS ยังใช้อยู่)
→ item ที่ไม่มี binary TRS: ใช้ fallback camera + ปิด PCA

---

## 4. ส่วน presets

ประกาศค่าตั้งต้นที่ใช้ซ้ำหลายที่ — ลดการพิมพ์ซ้ำ

```json
"presets": {
  "box": {
    "rotX": -75, "rotY": 15, "rotZ": 3.5,
    "camera": "noflip", "standardize": false
  },
  "jewel": {
    "rotX": -90, "rotY": 0, "rotZ": 0,
    "camera": "noflip", "standardize": false
  },
  "potion": {
    "rotX": -70, "rotY": 0, "rotZ": 0,
    "camera": "noflip", "standardize": false
  },
  "parchment": {
    "camera": "fallback", "standardize": false
  }
}
```

**การใช้งาน**: อ้างอิงด้วยชื่อ (string) ในส่วน sections, models, หรือ items:
```json
"items": {
  "14_1-9": "potion",
  "14_13-14": "jewel"
}
```
→ `"14_1-9": "potion"` = item 14_1 ถึง 14_9 ใช้ค่าจาก preset `potion`

---

## 5. ส่วน sections

กำหนดค่าทั้ง section — มีผลกับ item ทุกตัวใน section นั้น

```json
"sections": {
  "9":  { "camera": "fallback", "display_angle": -90, "bones": true, "override": true },
  "11": { "camera": "fallback", "display_angle": -90, "bones": true, "override": true },
  "12": { "display_angle": -90, "bones": true, "merge": true },
  "13": { "camera": "fallback", "standardize": false, "override": true },
  "16": { "camera": "fallback", "standardize": false, "override": true }
}
```

| Section | ประเภท | โหมด | เหตุผล |
|---------|--------|------|--------|
| 0 | ดาบ | override | re-tuning มุมใหม่ 60° |
| 1 | ขวาน | override | custom rotation + per-item |
| 9 | กางเกง | override | bones=true ประกอบ mesh ขา |
| 11 | รองเท้า | override | bones=true ประกอบ mesh เท้า |
| 12 | ปีก/pets | merge | เพิ่ม bones+DA เข้า binary TRS |
| 13 | อัญมณี | override | fallback camera ทั้ง section |
| 16 | Muuns | override | fallback camera ทั้ง section |

---

## 6. ส่วน models

กำหนดค่าตาม BMD model file — **ข้าม section ได้** (model เดียวกันในหลาย section ใช้ค่าเดียวกัน)

### รูปแบบที่ 1: Array (preset → list ของ model files)
```json
"models": {
  "box": [
    "itembox_gold.bmd", "itembox_silver.bmd",
    "RudeBox_blue.bmd", "RudeBox_red.bmd",
    "earringbox.bmd"
  ],
  "oldboot": [
    "BootMale01.bmd", "BootMale02.bmd", "BootMale03.bmd",
    "BootMale04.bmd", "BootMale05.bmd", "BootMale06.bmd",
    "BootMale07.bmd", "BootMale08.bmd", "BootMale09.bmd"
  ]
}
```
→ key `"box"` = ชื่อ preset → ทุก model ใน array ใช้ค่าจาก `presets.box`
→ `"oldboot"` = BootMale01-09 ใช้ preset oldboot (bones=false เพราะ mesh ง่าย)

### รูปแบบที่ 2: Single (model file → preset หรือ inline config)
```json
"models": {
  "coin7.bmd": { "rotX": -45, "rotY": 10, "camera": "noflip", "standardize": false }
}
```
→ item ทุกตัวที่ใช้ `coin7.bmd` จะได้ค่านี้ (ไม่ว่าอยู่ section ไหน)

**ประโยชน์**: กล่อง `RudeBox_red.bmd` อยู่ทั้ง section 14 (22 items) และ section 20 (15 items) — กำหนดใน models ครั้งเดียวได้ค่าเหมือนกันทั้งคู่

---

## 7. ส่วน items

กำหนดค่าเฉพาะ item — **ชนะทุกอย่าง** (สูงสุด)

### รูปแบบ key

| รูปแบบ | ตัวอย่าง | ขยายเป็น |
|--------|---------|---------|
| เดี่ยว | `"14_0"` | item section 14, index 0 |
| ช่วง | `"14_72-77"` | item 14_72, 14_73, 14_74, 14_75, 14_76, 14_77 |

### รูปแบบ value

| รูปแบบ | ตัวอย่าง | ความหมาย |
|--------|---------|---------|
| string | `"potion"` | อ้างอิง preset ชื่อ potion |
| object | `{ "rotX": -45, ... }` | กำหนดค่าโดยตรง |

**ตัวอย่าง**: ใช้ preset
```json
"items": {
  "14_1-9":     "potion",
  "14_13-14":   "jewel",
  "15_19-27":   "parchment"
}
```

**ตัวอย่าง**: กำหนดค่าโดยตรง
```json
"items": {
  "0_1": {
    "rotX": -45, "rotY": 30, "rotZ": 30,
    "fill_ratio": 0.80, "display_angle": -60,
    "bones": false, "override": true, "flip": false
  },
  "1_4": {
    "rotX": -5, "rotY": 40, "rotZ": 40,
    "scale": 0.0025, "camera": "noflip",
    "perspective": true, "fov": 45
  }
}
```

---

## 8. ตัวอย่างสถานการณ์จริง

### สถานการณ์ 1: render กล่องให้เห็นฝา+ด้านหน้า

**ปัญหา**: กล่อง itembox ใช้ fallback camera เห็นก้นกล่อง
**แก้ไข**: สร้าง preset `box` + ใช้ noflip camera ควบคุมมุมเอง

```json
"presets": {
  "box": { "rotX": -75, "rotY": 15, "rotZ": 3.5, "camera": "noflip", "standardize": false }
},
"models": {
  "box": ["itembox_gold.bmd", "itembox_silver.bmd", "earringbox.bmd"]
}
```
- `rotX: -75` = เอียงเห็นฝาด้านบน (-90=ตรงๆ, -75=เอียงหน้า 15°)
- `rotY: 15` = หมุนเห็นด้านข้างเล็กน้อย
- `rotZ: 3.5` = ชดเชย noflip camera ให้ฐานราบ
- `standardize: false` = ไม่ PCA rotate กล่องต้องอยู่มุมที่ตั้งไว้

---

### สถานการณ์ 2: ทำให้รองเท้าทั้ง section ตั้งตรง

**ปัญหา**: รองเท้าไม่มี bones → mesh ไม่ประกอบ + display_angle -45 ทำให้นอน
**แก้ไข**: section override + bones + display_angle -90

```json
"sections": {
  "11": { "camera": "fallback", "display_angle": -90, "bones": true, "override": true }
},
"models": {
  "oldboot": ["BootMale01.bmd", "BootMale02.bmd", ... "BootMale09.bmd"]
},
"presets": {
  "oldboot": { "camera": "fallback", "display_angle": -90, "bones": false }
}
```
- section 11: `override=true` → ทุก item ใช้ bones=true + ตั้งตรง
- `oldboot` models: override section ด้วย bones=false (mesh เก่า BMD v10 ไม่ต้อง bones)

---

### สถานการณ์ 3: ดาบที่ PCA จัดมุมผิด

**ปัญหา**: ดาบบางเล่ม PCA จับแกนผิด ปลายดาบชี้ผิดทาง
**แก้ไข**: per-item override ด้วย `flip: true` หรือ `flip_canvas: true`

```json
"items": {
  "0_5": {
    "rotX": -45, "rotY": 40,
    "fill_ratio": 0.90, "bones": false,
    "flip": true, "flip_canvas": true
  }
}
```
- `flip: true` = สลับทิศ PCA (ปลาย ↔ ด้าม)
- `flip_canvas: true` = พลิกภาพซ้าย-ขวาอีกชั้น
- ใส่ `bones: false` เสมอ! (per-item ไม่ inherit จาก section)

---

### สถานการณ์ 4: ปีกที่ต้อง merge ค่าเข้า binary TRS

**ปัญหา**: ปีก section 12 มี binary TRS (rotX=270, rotY=-10) ที่ดีอยู่แล้ว แต่ต้องเพิ่ม bones + display_angle
**แก้ไข**: ใช้ `merge` แทน `override` เพื่อเก็บ rotation เดิม

```json
"sections": {
  "12": { "display_angle": -90, "bones": true, "merge": true }
}
```
- `merge: true` → เก็บ rotX=270, rotY=-10 จาก binary TRS + เพิ่ม display_angle=-90, bones=true
- ถ้าใช้ `override` → ทับ rotation ด้วย 0,0,0 (ค่า default) → ภาพผิด!

---

### สถานการณ์ 5: item เฉพาะตัวที่ต้องการ perspective

**ปัญหา**: Crescent Axe (1_4) เป็น low-poly flat geometry → orthographic ทำให้เป็นเส้นตรง
**แก้ไข**: เปิด perspective + fov แคบ

```json
"items": {
  "1_4": {
    "rotX": -5, "rotY": 40, "rotZ": 40,
    "scale": 0.0025, "camera": "noflip",
    "perspective": true, "fov": 45
  }
}
```
- `perspective: true` = ฉาย 3D จริง (ใกล้ใหญ่ ไกลเล็ก)
- `fov: 45` = ซูมเข้าให้เห็น item ชัด

---

### สถานการณ์ 6: item ที่ render กลับด้านกับ reference

**ปัญหา**: เครื่องราง section 20 render ออกมาซ้าย-ขวากลับกับ BMD-viewer
**แก้ไข**: `flip_canvas: true`

```json
"presets": {
  "charm": { "camera": "fallback", "standardize": false, "flip_canvas": true }
},
"items": {
  "20_43-50": "charm",
  "20_162":   "charm"
}
```
- `flip_canvas: true` = mirror horizontal หลัง render เสร็จ
- ต่างจาก `flip` ที่สลับทิศ PCA — `flip_canvas` พลิกภาพจริงๆ

---

## สรุปตัวแปร (Quick Reference)

| ตัวแปร | ชนิด | ค่าเริ่มต้น | ใช้ใน | คำอธิบายสั้น |
|--------|------|------------|-------|-------------|
| `rotX` | float | 0 | ทุกที่ | หมุนรอบแกน X (องศา) |
| `rotY` | float | 0 | ทุกที่ | หมุนรอบแกน Y (องศา) |
| `rotZ` | float | 0 | ทุกที่ | หมุนรอบแกน Z (องศา) |
| `scale` | float | 0.0025 | ทุกที่ | ตัวคูณขนาด |
| `camera` | string | auto | ทุกที่ | กล้อง: noflip / correction / fallback |
| `bones` | bool | auto | ทุกที่ | เปิด/ปิด bone transforms |
| `standardize` | bool | true | ทุกที่ | เปิด/ปิด PCA จัดภาพ |
| `display_angle` | float | -45 | ทุกที่ | มุม PCA (องศา) |
| `fill_ratio` | float | 0.70 | ทุกที่ | สัดส่วนเติมผ้าใบ |
| `flip` | bool | false | ทุกที่ | สลับทิศ PCA |
| `flip_canvas` | bool | false | ทุกที่ | พลิกภาพซ้าย-ขวา |
| `perspective` | bool | false | ทุกที่ | เปิด perspective projection |
| `fov` | float | 75 | ทุกที่ | มุมมอง (ใช้กับ perspective) |
| `keep_all_meshes` | bool | false | ทุกที่ | ข้าม effect mesh filter |
| `override` | bool | false | sections | แทนที่ binary TRS ทั้ง section |
| `merge` | bool | false | sections | merge ค่าเข้า binary TRS |
