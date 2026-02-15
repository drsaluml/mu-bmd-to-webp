#!/usr/bin/env python3
"""
MU Online BMD 3D Model Renderer → WebP Item Images

Parses BMD (Binary Model Data) files, applies textures, renders isometric
projection, and saves as transparent WebP images.

Supports BMD versions: 10 (unencrypted), 12 (XOR), 15 (LEA-256 ECB).
"""

import struct
import math
import io
import os
import sys
import json
import re
import time
import xml.etree.ElementTree as ET
from pathlib import Path
from concurrent.futures import ProcessPoolExecutor, as_completed

import numpy as np
from PIL import Image, ImageFile

# Some OZJ files have minor JPEG stream issues — allow Pillow to decode them
ImageFile.LOAD_TRUNCATED_IMAGES = True

# ─── Configuration ────────────────────────────────────────────────────
BASE_DIR = Path(__file__).parent
ITEM_DIR = BASE_DIR / "Data" / "Item"
TEXTURE_DIR = BASE_DIR / "Data" / "Item-webp"
OUTPUT_DIR = BASE_DIR / "Data" / "Item-renders"
ITEMLIST_XML = BASE_DIR / "ItemList.xml"

TRSDATA_BMD = BASE_DIR / "Data" / "Local" / "itemtrsdata.bmd"
CUSTOM_TRS_JSON = BASE_DIR / "custom_trs.json"

RENDER_SIZE = 256
WEBP_QUALITY = 90
BACKGROUND = (0, 0, 0, 0)  # transparent

# ─── LEA-256 ECB Decryption ──────────────────────────────────────────
# Ported from: github.com/xulek/muonline-bmd-viewer/src/crypto/lea256.ts

_LEA_KEY = bytes([
    0xCC, 0x50, 0x45, 0x13, 0xC2, 0xA6, 0x57, 0x4E,
    0xD6, 0x9A, 0x45, 0x89, 0xBF, 0x2F, 0xBC, 0xD9,
    0x39, 0xB3, 0xB3, 0xBD, 0x50, 0xBD, 0xCC, 0xB6,
    0x85, 0x46, 0xD1, 0xD6, 0x16, 0x54, 0xE0, 0x87,
])

_KEY_DELTA = [
    0xC3EFE9DB, 0x44626B02, 0x79E27C8A, 0x78DF30EC,
    0x715EA49E, 0xC785DA0A, 0xE04EF22A, 0xE5C40957,
]

_XOR_KEY = bytes([
    0xD1, 0x73, 0x52, 0xF6, 0xD2, 0x9A, 0xCB, 0x27,
    0x3E, 0xAF, 0x59, 0x31, 0x37, 0xB3, 0xE7, 0xA2,
])


def _u32(x):
    return x & 0xFFFFFFFF


def _rol32(x, n):
    n &= 31
    return _u32((x << n) | (_u32(x) >> (32 - n)))


def _ror32(x, n):
    n &= 31
    return _u32((_u32(x) >> n) | (x << (32 - n)))


def _lea_key_schedule(key: bytes) -> list[int]:
    T = [struct.unpack_from("<I", key, i * 4)[0] for i in range(8)]
    rk = [0] * 192
    for i in range(32):
        d = _KEY_DELTA[i & 7]
        s = (i * 6) & 7
        shifts = [1, 3, 6, 11, 13, 17]
        for j, sh in enumerate(shifts):
            T[(s + j) & 7] = _rol32(_u32(T[(s + j) & 7] + _rol32(d, i + j)), sh)
        for j in range(6):
            rk[i * 6 + j] = T[(s + j) & 7]
    return rk


def _lea_decrypt_ecb(data: bytes, key: bytes) -> bytes:
    RK = _lea_key_schedule(key)
    out = bytearray(len(data))
    for off in range(0, len(data), 16):
        s = [struct.unpack_from("<I", data, off + i * 4)[0] for i in range(4)]
        for r in range(32):
            base = (31 - r) * 6
            rk = RK[base : base + 6]
            t0 = s[3]
            t1 = _u32(_ror32(s[0], 9) - _u32(t0 ^ rk[0]) ^ rk[1])
            t2 = _u32(_rol32(s[1], 5) - _u32(t1 ^ rk[2]) ^ rk[3])
            t3 = _u32(_rol32(s[2], 3) - _u32(t2 ^ rk[4]) ^ rk[5])
            s = [t0, t1, t2, t3]
        for i in range(4):
            struct.pack_into("<I", out, off + i * 4, s[i])
    return bytes(out)


def _xor_decrypt(data: bytes) -> bytes:
    out = bytearray(len(data))
    key = 0x5E
    for i in range(len(data)):
        out[i] = ((data[i] ^ _XOR_KEY[i & 15]) - key) & 0xFF
        key = (data[i] + 0x3D) & 0xFF
    return bytes(out)


# ─── BMD Parser ──────────────────────────────────────────────────────
# Ported from: github.com/xulek/muonline-bmd-viewer/src/bmd-loader.ts

def parse_bmd(filepath: str) -> tuple[list[dict], list[dict]]:
    """Parse a BMD file and return (meshes, bones).

    Each mesh dict has: verts, nodes, norms, uvs, tris, tex_path.
    Each bone dict has: parent, is_dummy, bind_position, bind_rotation.
    """
    with open(filepath, "rb") as f:
        raw = f.read()

    if raw[:3] != b"BMD":
        raise ValueError(f"Invalid BMD header: {filepath}")

    version = raw[3]

    if version in (12, 15):
        size = struct.unpack_from("<I", raw, 4)[0]
        enc = raw[8 : 8 + size]
        data = _lea_decrypt_ecb(enc, _LEA_KEY) if version == 15 else _xor_decrypt(enc)
    else:
        data = raw[4:]

    off = 0

    def read_str(length):
        nonlocal off
        s = data[off : off + length].split(b"\x00")[0].decode("ascii", errors="replace")
        off += length
        return s

    def read_i16():
        nonlocal off
        v = struct.unpack_from("<h", data, off)[0]
        off += 2
        return v

    def read_u16():
        nonlocal off
        v = struct.unpack_from("<H", data, off)[0]
        off += 2
        return v

    def read_f32():
        nonlocal off
        v = struct.unpack_from("<f", data, off)[0]
        off += 4
        return v

    _name = read_str(32)
    mesh_count = read_u16()
    bone_count = read_u16()
    action_count = read_u16()

    if mesh_count > 100 or mesh_count < 0:
        raise ValueError(f"Invalid mesh count {mesh_count} in {filepath}")

    meshes = []
    for _ in range(mesh_count):
        nv = read_i16()
        nn = read_i16()
        ntc = read_i16()
        nt = read_i16()
        _tex_idx = read_i16()

        # Vertices: 16 bytes each (node:i16, pad:i16, x:f32, y:f32, z:f32)
        verts = np.zeros((nv, 3), dtype=np.float32)
        nodes = np.zeros(nv, dtype=np.int16)
        for i in range(nv):
            nodes[i] = read_i16()
            read_i16()  # padding
            verts[i, 0] = read_f32()
            verts[i, 1] = read_f32()
            verts[i, 2] = read_f32()

        # Normals: 20 bytes each (node:i16, pad:i16, nx:f32, ny:f32, nz:f32, bind:i16, pad:i16)
        norms = np.zeros((nn, 3), dtype=np.float32)
        for i in range(nn):
            read_i16()  # node
            read_i16()  # padding
            norms[i, 0] = read_f32()
            norms[i, 1] = read_f32()
            norms[i, 2] = read_f32()
            read_i16()  # bindVertex
            read_i16()  # padding

        # TexCoords: 8 bytes each (u:f32, v:f32)
        uvs = np.zeros((ntc, 2), dtype=np.float32)
        for i in range(ntc):
            uvs[i, 0] = read_f32()
            uvs[i, 1] = read_f32()

        # Triangles: 64 bytes each
        tris = []
        for _ in range(nt):
            base = off
            poly = data[base]
            vi = [struct.unpack_from("<h", data, base + 2 + j * 2)[0] for j in range(4)]
            ni = [struct.unpack_from("<h", data, base + 10 + j * 2)[0] for j in range(4)]
            ti = [struct.unpack_from("<h", data, base + 18 + j * 2)[0] for j in range(4)]
            tris.append((poly, vi, ni, ti))
            off += 64

        tex_path = read_str(32)

        meshes.append({
            "verts": verts,
            "nodes": nodes,
            "norms": norms,
            "uvs": uvs,
            "tris": tris,
            "tex_path": tex_path,
        })

    # ── Parse actions ──
    actions_keys = []
    for _a in range(action_count):
        num_keys = read_i16()
        lock_pos = data[off] > 0
        off += 1
        if lock_pos:
            off += num_keys * 12  # skip float32 x,y,z per key
        actions_keys.append(num_keys)

    # ── Parse bones ──
    bones = []
    for _b in range(bone_count):
        is_dummy = data[off] > 0
        off += 1
        if is_dummy:
            bones.append({'parent': -1, 'is_dummy': True,
                          'bind_position': (0, 0, 0), 'bind_rotation': (0, 0, 0)})
            continue

        _bone_name = read_str(32)
        parent = read_i16()

        bind_pos = (0.0, 0.0, 0.0)
        bind_rot = (0.0, 0.0, 0.0)
        for a_idx in range(action_count):
            num_keys = actions_keys[a_idx] if a_idx < len(actions_keys) else 0
            if num_keys == 0:
                continue
            # positions: num_keys × (float32 x, y, z)
            positions = []
            for _k in range(num_keys):
                positions.append((read_f32(), read_f32(), read_f32()))
            # rotations: num_keys × (float32 rx, ry, rz)
            rotations = []
            for _k in range(num_keys):
                rotations.append((read_f32(), read_f32(), read_f32()))
            # Use frame 0 of action 0 as bind pose
            if a_idx == 0:
                bind_pos = positions[0]
                bind_rot = rotations[0]

        bones.append({
            'parent': parent,
            'is_dummy': False,
            'bind_position': bind_pos,
            'bind_rotation': bind_rot,
        })

    return meshes, bones


# ─── Texture Resolver ────────────────────────────────────────────────

# Build a case-insensitive lookup index: stem.lower() → full path
# OZJ = JPEG, OZT = TGA — both readable by Pillow directly
_tex_index: dict[str, str] = {}


def _build_texture_index():
    """Scan all texture directories once and build stem→path lookup."""
    if _tex_index:
        return

    search_dirs = [ITEM_DIR / "texture"]
    # Also scan subdirectory textures (Jewel/Texture, partCharge1/Texture, etc.)
    if ITEM_DIR.exists():
        for sub in ITEM_DIR.iterdir():
            if sub.is_dir():
                for tex_sub in ["Texture", "texture"]:
                    d = sub / tex_sub
                    if d.exists():
                        search_dirs.append(d)

    for search_dir in search_dirs:
        if not search_dir.exists():
            continue
        for f in search_dir.iterdir():
            if f.suffix.lower() in (".ozj", ".ozt"):
                key = f.stem.lower()
                ext = f.suffix.lower()
                if key not in _tex_index:
                    _tex_index[key] = str(f)
                elif ext == ".ozt" and Path(_tex_index[key]).suffix.lower() == ".ozj":
                    # Prefer OZT (TGA with alpha) over OZJ (JPEG, no alpha)
                    _tex_index[key] = str(f)


def _resolve_texture(tex_name: str) -> str | None:
    """Find the OZJ/OZT texture file for a given BMD texture reference."""
    if not tex_name:
        return None

    _build_texture_index()

    # Strip path prefix, normalize: "sword02.jpg" → "sword02"
    base = tex_name.replace("\\", "/").split("/")[-1]
    stem = Path(base).stem.lower()

    return _tex_index.get(stem)


_tex_cache: dict[str, np.ndarray | None] = {}


def _load_texture(tex_name: str) -> np.ndarray | None:
    """Load texture as RGBA numpy array, cached.

    OZJ = 24-byte header + JPEG data.
    OZT = 4-byte header + TGA data.
    """
    if tex_name in _tex_cache:
        return _tex_cache[tex_name]

    path = _resolve_texture(tex_name)
    if path is None:
        _tex_cache[tex_name] = None
        return None

    try:
        with open(path, "rb") as f:
            data = f.read()

        ext = Path(path).suffix.lower()
        if ext == ".ozj":
            data = data[24:]  # skip OZJ header
        elif ext == ".ozt":
            data = data[4:]   # skip OZT header

        img = Image.open(io.BytesIO(data)).convert("RGBA")
        arr = np.array(img, dtype=np.float32)
        _tex_cache[tex_name] = arr
        return arr
    except Exception:
        _tex_cache[tex_name] = None
        return None


# ─── Effect Mesh Filter ───────────────────────────────────────────────

_EFFECT_PATTERNS = [
    'glow', 'blur', 'flare', 'fire', 'flame', 'lightning',
    'elec_light', 'arrowlight', 'lighting_mega',
    'effect', 'lightmarks', 'light_blue', 'light_red',
    'shiny', 'spark', 'aura', 'energy', 'plasma',
    'chrome', 'shine', 'halo', 'trail',
    'gradation',  # gradient glow (additive blending)
]

# Gradient textures for additive blending: gra, gra2, gra3_1, mini_gra, hangulgra_r, etc.
# These render as dark blobs without additive blend support.
_GRADIENT_EFFECT_RE = re.compile(r'^(?:mini_|hangul)?gra(?:\d|_|$)')


def _is_effect_mesh(mesh: dict) -> bool:
    """Return True if this mesh is an aura / glow / effect overlay."""
    nv = len(mesh['verts'])
    nt = len(mesh['tris'])
    tex = mesh['tex_path'].lower()
    stem = Path(tex.replace('\\', '/')).stem

    # Texture-based detection (always filter these)
    if _GRADIENT_EFFECT_RE.match(stem):
        return True
    if any(p in tex for p in _EFFECT_PATTERNS):
        return True

    # Small geometry heuristic — but keep large quads (e.g. blade decals)
    if nv <= 8 and nt <= 4:
        verts = mesh['verts']
        if len(verts) > 0:
            span = (verts.max(axis=0) - verts.min(axis=0)).max()
            if span > 20:  # significant visual area → not effect
                return False
        return True

    return False


# ─── Connected Component Filter ──────────────────────────────────────

def _filter_mesh_components(mesh: dict, min_verts: int = 6) -> dict:
    """Remove small disconnected components (floating artifact quads)."""
    from collections import defaultdict
    verts = mesh['verts']
    tris = mesh['tris']
    if len(verts) == 0 or len(tris) == 0:
        return mesh

    adj = defaultdict(set)
    for poly, vi, ni, ti in tris:
        indices = vi[:4] if poly == 4 else vi[:3]
        for a in range(len(indices)):
            for b in range(a + 1, len(indices)):
                if 0 <= indices[a] < len(verts) and 0 <= indices[b] < len(verts):
                    adj[indices[a]].add(indices[b])
                    adj[indices[b]].add(indices[a])

    visited = set()
    components = []
    for v in range(len(verts)):
        if v in visited or v not in adj:
            continue
        comp = set()
        stack = [v]
        while stack:
            curr = stack.pop()
            if curr in visited:
                continue
            visited.add(curr)
            comp.add(curr)
            for n in adj[curr]:
                if n not in visited:
                    stack.append(n)
        components.append(comp)

    if len(components) <= 1:
        return mesh

    largest = max(components, key=len)
    largest_center = verts[list(largest)].mean(axis=0)
    largest_span = max(verts[list(largest)].max(axis=0) - verts[list(largest)].min(axis=0))

    keep_verts = set(largest)
    for comp in components:
        if comp is largest:
            continue
        if len(comp) >= min_verts:
            keep_verts |= comp
        else:
            comp_center = verts[list(comp)].mean(axis=0)
            dist = np.linalg.norm(comp_center - largest_center)
            if dist < largest_span * 0.4:
                keep_verts |= comp

    filtered_tris = [t for t in tris if all(v in keep_verts for v in t[1][:3])]
    return {**mesh, 'tris': filtered_tris}


# ─── View Transform ──────────────────────────────────────────────────
# Per-item orientation from ItemTRSData.bmd (game client display transforms)
# converted to match muonline-bmd-viewer (Three.js) reference renders.
#
# MU Online uses DirectX (left-handed). Our renderer is right-handed.
# CORRECTION matrix maps TRS default rotation → BMD-viewer camera output.
#
# BMD-viewer setup:
#   Model: group.rotation.x = -PI/2 (Z-up → Y-up)
#   Camera: position (0, 200, 400), target (0, 90, 0) → ~15° elevation
#   Lights: Directional(1.7) at (180,260,140), Rim(0.72) at (-160,130,-210)
#           Ambient(0.42), Hemisphere(0.52)

def _Rx(a: float) -> np.ndarray:
    c, s = math.cos(a), math.sin(a)
    return np.array([[1,0,0],[0,c,-s],[0,s,c]], dtype=np.float64)

def _Ry(a: float) -> np.ndarray:
    c, s = math.cos(a), math.sin(a)
    return np.array([[c,0,s],[0,1,0],[-s,0,c]], dtype=np.float64)

def _Rz(a: float) -> np.ndarray:
    c, s = math.cos(a), math.sin(a)
    return np.array([[c,-s,0],[s,c,0],[0,0,1]], dtype=np.float64)

_MODEL_FLIP = _Rx(math.radians(-90))  # Z-up → Y-up
_MIRROR_X = np.diag([-1.0, 1.0, 1.0])  # DirectX LH → OpenGL RH

# Fallback view (BMD-viewer match) for items without TRS data
_VIEW_FALLBACK = _MIRROR_X @ _Rx(math.radians(-15)) @ _Ry(math.radians(12)) @ _MODEL_FLIP

# ─── TRS Correction Matrix ──────────────────────────────────────────
# CORRECTION = BMD_VIEW_MATCH @ inv(TRS_DEFAULT_ROTATION)
# Maps game-client TRS rotations → BMD-viewer-style output.
# Default weapon TRS: rotX=180, rotY=270, rotZ=15
_TRS_DEFAULT = _Rz(math.radians(15)) @ _Ry(math.radians(270)) @ _Rx(math.radians(180))
_TRS_CORRECTION = _VIEW_FALLBACK @ np.linalg.inv(_TRS_DEFAULT)

# Lighting system matching BMD-viewer (Three.js)
# BMD-viewer uses: Ambient(0.42) + Hemisphere(0.52) + Directional(1.7) + Rim(0.72)
# + MeshPhongMaterial(specular) + ACES Filmic tone mapping(0.95)
_LIGHT_DIR = np.array([180.0, 260.0, 140.0], dtype=np.float64)
_LIGHT_DIR /= np.linalg.norm(_LIGHT_DIR)

_RIM_DIR = np.array([-160.0, 130.0, -210.0], dtype=np.float64)
_RIM_DIR /= np.linalg.norm(_RIM_DIR)

# View direction (camera look-at, normalized) for specular
_VIEW_DIR = np.array([0.0, -110.0, -400.0], dtype=np.float64)  # from (0,200,400) to (0,90,0)
_VIEW_DIR /= np.linalg.norm(_VIEW_DIR)

_AMBIENT = 0.55           # BMD-viewer: 0.42 + IBL ~0.15 fill (we lack IBL, boost ambient)
_HEMI_INTENSITY = 0.50    # BMD-viewer: 0.52 + IBL fill
_DIRECT_INTENSITY = 1.50  # BMD-viewer: 1.7 (slightly lower to reduce contrast)
_RIM_INTENSITY = 0.60     # BMD-viewer: 0.72
_SPECULAR_INTENSITY = 0.45
_SPECULAR_POWER = 12.0    # shininess (BMD-viewer uses ≥12)
_TONEMAP_EXPOSURE = 1.05  # ACES Filmic exposure (slightly over 1 to brighten)

# sRGB gamma for linear color space conversion
_SRGB_GAMMA = 2.2


# ─── ItemTRSData.bmd Loader ─────────────────────────────────────────

_TRS_XOR_KEY = bytes([0xfc, 0xcf, 0xab])

def _xor_decrypt_trs(data: bytes) -> bytes:
    out = bytearray(len(data))
    for i in range(len(data)):
        out[i] = data[i] ^ _TRS_XOR_KEY[i % 3]
    return bytes(out)


def load_trs_data(filepath: Path = TRSDATA_BMD) -> dict[tuple[int,int], dict]:
    """Load ItemTRSData.bmd + custom_trs.json → dict keyed by (section, index).

    Priority: custom_trs.json items > custom_trs.json sections > binary TRS.
    """
    trs: dict[tuple[int,int], dict] = {}

    # ── Binary TRS (game client data) ──
    if filepath.exists():
        raw = filepath.read_bytes()
        count = struct.unpack_from("<I", raw, 0)[0]
        offset = 4

        for _ in range(count):
            enc = raw[offset:offset + 32]
            dec = _xor_decrypt_trs(enc)
            item_id = struct.unpack_from("<I", dec, 0)[0]
            px, py, pz = struct.unpack_from("<fff", dec, 4)
            rx, ry, rz = struct.unpack_from("<fff", dec, 16)
            scale = struct.unpack_from("<f", dec, 28)[0]

            section = item_id // 512
            index = item_id % 512
            trs[(section, index)] = {
                'posX': px, 'posY': py, 'posZ': pz,
                'rotX': rx, 'rotY': ry, 'rotZ': rz,
                'scale': scale,
                '_source': 'binary',
            }
            offset += 32

    # ── Custom TRS overrides (JSON) ──
    trs = _merge_custom_trs(trs)

    return trs


def _make_trs_entry(obj: dict) -> dict:
    """Normalize a JSON TRS object to a full TRS entry with defaults."""
    entry = {
        'posX': obj.get('posX', 0.0), 'posY': obj.get('posY', 0.0), 'posZ': obj.get('posZ', 0.0),
        'rotX': obj.get('rotX', 0.0), 'rotY': obj.get('rotY', 0.0), 'rotZ': obj.get('rotZ', 0.0),
        'scale': obj.get('scale', 0.0),
        '_source': 'custom',
    }
    for key in ('bones', 'display_angle', 'flip', 'fill_ratio', 'camera', 'perspective', 'fov'):
        if key in obj:
            entry[key] = obj[key]
    return entry


def _merge_custom_trs(trs: dict[tuple[int,int], dict],
                      filepath: Path = CUSTOM_TRS_JSON) -> dict[tuple[int,int], dict]:
    """Merge custom_trs.json into TRS dict.

    JSON format:
      {
        "sections": { "<section>": { rotX, rotY, rotZ, scale, ... } },
        "items":    { "<section>_<index>": { rotX, rotY, rotZ, scale, ... } }
      }

    "sections" applies to ALL items in that section that don't have binary TRS.
    Use "override": true in a section to also replace binary TRS entries.
    "items" overrides everything (binary + section defaults).
    """
    if not filepath.exists():
        return trs

    try:
        with open(filepath, "r", encoding="utf-8") as f:
            data = json.load(f)
    except (json.JSONDecodeError, OSError) as e:
        print(f"Warning: custom_trs.json load failed: {e}")
        return trs

    # Section defaults/overrides
    for sec_str, val in data.get("sections", {}).items():
        sec = int(sec_str)
        entry = _make_trs_entry(val)
        override = val.get("override", False)
        import xml.etree.ElementTree as _ET
        try:
            _tree = _ET.parse(str(ITEMLIST_XML))
            for section_el in _tree.getroot().findall("Section"):
                if int(section_el.get("Index")) == sec:
                    for item_el in section_el.findall("Item"):
                        idx = int(item_el.get("Index"))
                        if override or (sec, idx) not in trs:
                            trs[(sec, idx)] = entry.copy()
        except Exception:
            pass

    # Per-item overrides — always wins
    for key_str, val in data.get("items", {}).items():
        parts = key_str.split("_", 1)
        if len(parts) == 2:
            sec, idx = int(parts[0]), int(parts[1])
            trs[(sec, idx)] = _make_trs_entry(val)

    return trs


def _angle_dist(a: float, b: float) -> float:
    """Shortest angular distance between two angles in degrees."""
    d = (a - b) % 360
    return d if d <= 180 else 360 - d


# Noflip camera for items where correction matrix produces edge-on views.
# Used when rotY is far from the default weapon rotY=270.
_NOFLIP_CAM = _MIRROR_X @ _Rx(math.radians(-15))


def _trs_view_matrix(trs_entry: dict) -> np.ndarray:
    """Build view matrix from TRS rotation.

    Uses hybrid approach:
    - Correction matrix for items with rotY near 270° or 90° (weapons, shields, spears)
    - Direct noflip approach for other items (wings, armor, misc)

    The correction matrix was calibrated against the default weapon TRS (180,270,15).
    Items with very different rotY values (wings at -10°, misc at 0°) get edge-on views
    with correction, so they use a direct camera approach without MODEL_FLIP instead.
    """
    rx = math.radians(trs_entry['rotX'])
    ry = math.radians(trs_entry['rotY'])
    rz = math.radians(trs_entry['rotZ'])
    trs_rot = _Rz(rz) @ _Ry(ry) @ _Rx(rx)

    # Allow custom TRS to force camera path: "camera": "noflip" or "correction"
    cam = trs_entry.get('camera')
    if cam == 'noflip':
        return _NOFLIP_CAM @ trs_rot
    elif cam == 'correction':
        return _TRS_CORRECTION @ trs_rot

    rotY = trs_entry['rotY']
    if _angle_dist(rotY, 270) <= 45 or _angle_dist(rotY, 90) <= 45:
        return _TRS_CORRECTION @ trs_rot
    else:
        return _NOFLIP_CAM @ trs_rot


def _compute_view_matrix(meshes: list[dict], trs_entry: dict | None = None) -> tuple[np.ndarray, list[dict]]:
    """Return (view_matrix, filtered_meshes).

    If trs_entry is provided, uses per-item TRS rotation (preferred).
    Otherwise falls back to the default view.
    """
    # Debug: show all meshes and which are filtered
    for i, m in enumerate(meshes):
        nv, nt = len(m['verts']), len(m['tris'])
        tex = Path(m['tex_path'].replace('\\', '/')).stem if m.get('tex_path') else '?'
        is_eff = _is_effect_mesh(m)
        print(f"    mesh[{i}] verts={nv} tris={nt} tex={tex} {'FILTERED' if is_eff else 'KEEP'}")
    body = [m for m in meshes if not _is_effect_mesh(m)]
    if not body:
        body = meshes
    body = [_filter_mesh_components(m) for m in body]

    if trs_entry is not None:
        return _trs_view_matrix(trs_entry), body

    return _VIEW_FALLBACK, body


# ─── Post-Processing ─────────────────────────────────────────────────

def _remove_small_clusters(img: Image.Image, min_ratio: float = 0.02) -> Image.Image:
    """Remove small disconnected pixel clusters (floating artifacts)."""
    arr = np.array(img)
    alpha = arr[:, :, 3]
    mask = alpha > 0
    if not mask.any():
        return img

    h, w = mask.shape
    labels = np.zeros((h, w), dtype=np.int32)
    label_id = 0
    sizes = []

    # 8-connected flood fill
    for y in range(h):
        for x in range(w):
            if mask[y, x] and labels[y, x] == 0:
                label_id += 1
                stack = [(y, x)]
                labels[y, x] = label_id
                count = 0
                while stack:
                    cy, cx = stack.pop()
                    count += 1
                    for dy in (-1, 0, 1):
                        for dx in (-1, 0, 1):
                            if dy == 0 and dx == 0:
                                continue
                            ny, nx = cy + dy, cx + dx
                            if 0 <= ny < h and 0 <= nx < w and mask[ny, nx] and labels[ny, nx] == 0:
                                labels[ny, nx] = label_id
                                stack.append((ny, nx))
                sizes.append(count)

    if len(sizes) <= 1:
        return img

    total = sum(sizes)
    for i, size in enumerate(sizes):
        if size < total * min_ratio:
            arr[labels == (i + 1)] = [0, 0, 0, 0]

    return Image.fromarray(arr, "RGBA")


# ─── Item Standardization (2D Post-Processing) ──────────────────────
# Standardize all item icons: fixed -45° diagonal, 70% canvas, centered.
# Uses PCA on non-transparent pixels to find principal axis, then rotates,
# scales, and centers. Ensures wider end (blade/head) is at top-left.

def _standardize_item_image(img: Image.Image, size: int = RENDER_SIZE,
                             target_angle_deg: float = -45.0,
                             fill_ratio: float = 0.70,
                             force_flip: bool = False) -> Image.Image:
    """Standardize item icon: rotate to fixed angle, scale, center.

    Args:
        img: Rendered RGBA image
        size: Output canvas size (square)
        target_angle_deg: Target angle in math convention
                          (-45° = top-left to bottom-right diagonal)
        fill_ratio: Max dimension as fraction of canvas size (0.70 = 70%)
    """
    arr = np.array(img)
    alpha = arr[:, :, 3]
    ys, xs = np.where(alpha > 0)
    if len(xs) < 10:
        return img

    # PCA in image coordinates (x→right, y→down)
    coords = np.column_stack([xs.astype(np.float64), ys.astype(np.float64)])
    mean = coords.mean(axis=0)
    centered = coords - mean
    cov = np.cov(centered.T)
    eigenvalues, eigenvectors = np.linalg.eigh(cov)

    principal = eigenvectors[:, np.argmax(eigenvalues)]
    current_angle = math.degrees(math.atan2(principal[1], principal[0]))

    # In image coords (y-down): +45° = "\" diagonal = -45° in math convention
    target_img = -target_angle_deg  # -(-45) = +45° in image coords

    # PIL.rotate(θ) CCW → new_angle = old_angle - θ → θ = old - target
    pil_angle = current_angle - target_img
    while pil_angle > 90:
        pil_angle -= 180
    while pil_angle < -90:
        pil_angle += 180

    print(f"  [standardize] PCA_angle={current_angle:.1f}° target_math={target_angle_deg}° target_img={target_img:.1f}° pil_rotate={pil_angle:.1f}° flip={force_flip}")

    rotated = img.rotate(pil_angle, resample=Image.BICUBIC, expand=True,
                          fillcolor=(0, 0, 0, 0))

    # Verify blade orientation: wider end should be at top-left
    rot_arr = np.array(rotated)
    rot_alpha = rot_arr[:, :, 3]
    rot_ys, rot_xs = np.where(rot_alpha > 0)
    if len(rot_xs) == 0:
        return img

    cx = (rot_xs.min() + rot_xs.max()) / 2.0
    cy = (rot_ys.min() + rot_ys.max()) / 2.0

    diag = np.array([math.cos(math.radians(target_img)),
                     math.sin(math.radians(target_img))])
    proj = (rot_xs - cx) * diag[0] + (rot_ys - cy) * diag[1]
    perp = -(rot_xs - cx) * diag[1] + (rot_ys - cy) * diag[0]

    neg_half = proj < 0   # top-left half
    pos_half = proj >= 0  # bottom-right half
    spread_tl = np.std(perp[neg_half]) if neg_half.sum() > 10 else 0
    spread_br = np.std(perp[pos_half]) if pos_half.sum() > 10 else 0

    need_flip = spread_br > spread_tl * 1.2
    if force_flip:
        need_flip = not need_flip  # invert the auto-detected orientation

    if need_flip:
        rotated = rotated.rotate(180, resample=Image.BICUBIC,
                                  fillcolor=(0, 0, 0, 0))
        rot_arr = np.array(rotated)
        rot_alpha = rot_arr[:, :, 3]
        rot_ys, rot_xs = np.where(rot_alpha > 0)

    # Crop to content bounding box
    x1, x2 = int(rot_xs.min()), int(rot_xs.max())
    y1, y2 = int(rot_ys.min()), int(rot_ys.max())
    cw = x2 - x1 + 1
    ch = y2 - y1 + 1
    cropped = rotated.crop((x1, y1, x2 + 1, y2 + 1))

    # Scale: max dimension = fill_ratio × canvas
    target_px = int(size * fill_ratio)
    sf = target_px / max(cw, ch) if max(cw, ch) > 0 else 1.0
    nw = max(1, int(cw * sf))
    nh = max(1, int(ch * sf))
    scaled = cropped.resize((nw, nh), Image.LANCZOS)

    # Center on canvas
    result = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    result.paste(scaled, ((size - nw) // 2, (size - nh) // 2))
    return result


# ─── Bone Transform Pipeline ────────────────────────────────────────
# BMD vertices are stored in "neutral" space. The BMD-viewer (Three.js)
# calls SkinnedMesh.bind() BEFORE positioning bones, so boneInverses
# are identity. Then bones are moved to bind pose, making the skinning
# formula: v_world = boneWorldMatrix × v_model.
# We replicate this: compute bone world matrices from bind pose and
# transform vertices before applying the view matrix.

def _euler_to_quat(rx, ry, rz):
    """Euler XYZ (radians) → quaternion (x,y,z,w). Matches bmdAngleToQuaternion."""
    hx, hy, hz = rx * 0.5, ry * 0.5, rz * 0.5
    sx, cx = math.sin(hx), math.cos(hx)
    sy, cy = math.sin(hy), math.cos(hy)
    sz, cz = math.sin(hz), math.cos(hz)
    w = cx*cy*cz + sx*sy*sz
    x = sx*cy*cz - cx*sy*sz
    y = cx*sy*cz + sx*cy*sz
    z = cx*cy*sz - sx*sy*cz
    ln = math.sqrt(w*w + x*x + y*y + z*z)
    if ln > 0:
        x, y, z, w = x/ln, y/ln, z/ln, w/ln
    return x, y, z, w


def _quat_to_mat33(qx, qy, qz, qw):
    """Quaternion → 3×3 rotation matrix."""
    xx, yy, zz = qx*qx, qy*qy, qz*qz
    xy, xz, yz = qx*qy, qx*qz, qy*qz
    wx, wy, wz = qw*qx, qw*qy, qw*qz
    return np.array([
        [1-2*(yy+zz), 2*(xy-wz), 2*(xz+wy)],
        [2*(xy+wz), 1-2*(xx+zz), 2*(yz-wx)],
        [2*(xz-wy), 2*(yz+wx), 1-2*(xx+yy)],
    ], dtype=np.float64)


def _build_bone_world_matrices(bones):
    """Build 4×4 world matrices for each bone at bind pose (frame 0, action 0)."""
    n = len(bones)
    worlds = [np.eye(4, dtype=np.float64) for _ in range(n)]

    for i in range(n):
        bone = bones[i]
        if bone['is_dummy']:
            continue  # stays identity

        pos = bone['bind_position']
        rot = bone['bind_rotation']
        qx, qy, qz, qw = _euler_to_quat(*rot)
        R = _quat_to_mat33(qx, qy, qz, qw)

        local = np.eye(4, dtype=np.float64)
        local[:3, :3] = R
        local[:3, 3] = pos

        parent = bone['parent']
        if 0 <= parent < n and parent != i:
            worlds[i] = worlds[parent] @ local
        else:
            worlds[i] = local

    return worlds


def _apply_bone_transforms(meshes, bones):
    """Transform mesh vertices by their bone's world matrix (bind pose)."""
    if not bones:
        return

    worlds = _build_bone_world_matrices(bones)
    n_bones = len(worlds)

    # Quick check: if all bones are identity, skip
    has_transform = any(
        not np.allclose(worlds[i], np.eye(4), atol=1e-6)
        for i in range(n_bones)
    )
    if not has_transform:
        return

    for mesh in meshes:
        verts = mesh['verts']
        nodes = mesh.get('nodes')
        if nodes is None or len(verts) == 0:
            continue

        for i in range(len(verts)):
            bi = int(nodes[i])
            if bi < 0 or bi >= n_bones:
                continue
            M = worlds[bi]
            v = np.array([verts[i, 0], verts[i, 1], verts[i, 2], 1.0], dtype=np.float64)
            vt = M @ v
            verts[i] = vt[:3].astype(np.float32)


# ─── Software Rasterizer ─────────────────────────────────────────────

def _sample_texture(tex: np.ndarray, u: float, v: float) -> tuple[int, int, int, int]:
    """Sample texture at UV coordinates with bilinear filtering and wrapping."""
    h, w = tex.shape[:2]
    u = u % 1.0
    v = v % 1.0
    fx = u * (w - 1)
    fy = v * (h - 1)
    x0 = int(fx)
    y0 = int(fy)
    x1 = (x0 + 1) % w
    y1 = (y0 + 1) % h
    dx = fx - x0
    dy = fy - y0
    c = (tex[y0, x0] * (1 - dx) * (1 - dy) +
         tex[y0, x1] * dx * (1 - dy) +
         tex[y1, x0] * (1 - dx) * dy +
         tex[y1, x1] * dx * dy)
    return int(c[0] + 0.5), int(c[1] + 0.5), int(c[2] + 0.5), int(c[3] + 0.5)


def render_bmd(meshes: list[dict], size: int = RENDER_SIZE,
               trs_entry: dict | None = None,
               bones: list[dict] | None = None,
               standardize: bool = True,
               supersample: int = 2) -> Image.Image:
    """Render parsed BMD meshes to a PIL Image with textured triangles.

    If trs_entry is provided, uses per-item TRS rotation from ItemTRSData.bmd.
    Otherwise falls back to default view orientation.
    Bone transforms are applied only when TRS is absent or custom (not binary).
    Binary TRS data was calibrated for raw mesh without skeleton.
    supersample: render at N× resolution then downsample (anti-aliasing).
    """
    use_bones = trs_entry is None or trs_entry.get('_source') != 'binary'
    if trs_entry and trs_entry.get('bones') is False:
        use_bones = False
    bones_count = len(bones) if bones else 0
    max_node = -1
    for m in meshes:
        ns = m.get('nodes')
        if ns is not None and len(ns) > 0:
            max_node = max(max_node, int(np.max(ns)))
    print(f"    use_bones={use_bones} bones_count={bones_count} max_bone_idx={max_node} source={trs_entry.get('_source') if trs_entry else 'N/A'}")
    if use_bones:
        _apply_bone_transforms(meshes, bones)

    # Debug: bounding box after bone transforms
    for i, m in enumerate(meshes):
        v = m['verts']
        if len(v) > 0:
            mn = v.min(axis=0)
            mx = v.max(axis=0)
            span = mx - mn
            print(f"    mesh[{i}] bbox: X={span[0]:.1f} Y={span[1]:.1f} Z={span[2]:.1f}  (bones={'ON' if use_bones else 'OFF'})")

    R, body_meshes = _compute_view_matrix(meshes, trs_entry)

    all_verts = np.vstack([m["verts"] for m in body_meshes if len(m["verts"]) > 0])
    if len(all_verts) == 0:
        return Image.new("RGBA", (size, size), BACKGROUND)

    render_size = size * supersample

    all_transformed = (R @ all_verts.T).T

    # Perspective projection (per-item opt-in)
    use_persp = bool(trs_entry.get('perspective', False)) if trs_entry else False
    persp_cam_dist = 0.0
    persp_z_center = 0.0
    if use_persp:
        fov = trs_entry.get('fov', 75.0) if trs_entry else 75.0
        half_fov = math.radians(fov / 2)
        z_mn, z_mx = float(all_transformed[:, 2].min()), float(all_transformed[:, 2].max())
        persp_z_center = (z_mn + z_mx) / 2
        xy_half = max((all_transformed[:, :2].max(axis=0) - all_transformed[:, :2].min(axis=0)).max() / 2, 0.001)
        persp_cam_dist = xy_half / math.tan(half_fov)
        # Apply: closer to camera (higher z) appears larger
        z_off = all_transformed[:, 2] - persp_z_center
        depth = np.maximum(persp_cam_dist - z_off, 0.1)
        factor = persp_cam_dist / depth
        all_transformed[:, 0] *= factor
        all_transformed[:, 1] *= factor

    mn = all_transformed.min(axis=0)
    mx = all_transformed.max(axis=0)
    center = (mn + mx) / 2
    span = max((mx - mn)[:2].max(), 0.001)

    margin = 16 * supersample
    scale = (render_size - 2 * margin) / span

    color_buf = np.zeros((render_size, render_size, 4), dtype=np.uint8)
    z_buf = np.full((render_size, render_size), -np.inf, dtype=np.float64)

    for mesh in body_meshes:
        verts = mesh["verts"]
        uvs = mesh["uvs"]
        tris = mesh["tris"]
        tex_path = mesh["tex_path"]

        if len(verts) == 0:
            continue

        verts_t = (R @ verts.T).T
        if use_persp:
            z_off = verts_t[:, 2] - persp_z_center
            depth = np.maximum(persp_cam_dist - z_off, 0.1)
            factor = persp_cam_dist / depth
            verts_t = verts_t.copy()
            verts_t[:, 0] *= factor
            verts_t[:, 1] *= factor
        px = ((verts_t[:, 0] - center[0]) * scale + render_size / 2).astype(np.float64)
        py = (-(verts_t[:, 1] - center[1]) * scale + render_size / 2).astype(np.float64)
        pz = verts_t[:, 2].astype(np.float64)

        tex = _load_texture(tex_path)
        if tex is not None:
            avg = tex[:, :, :3].mean(axis=(0, 1)).astype(int)
            default_color = (int(avg[0]), int(avg[1]), int(avg[2]), 255)
        else:
            default_color = (160, 160, 170, 255)

        for poly, vi, _ni, ti in tris:
            _rasterize_triangle(
                px, py, pz, uvs, vi, ti, tex, default_color,
                color_buf, z_buf, render_size,
            )
            if poly == 4:
                _rasterize_triangle(
                    px, py, pz, uvs, [vi[0], vi[2], vi[3]], [ti[0], ti[2], ti[3]],
                    tex, default_color, color_buf, z_buf, render_size,
                )

    img = Image.fromarray(color_buf, "RGBA")
    if supersample > 1:
        # Premultiply alpha before downsample to prevent dark edge halos
        arr = np.array(img, dtype=np.float32)
        alpha = arr[:, :, 3:4] / 255.0
        arr[:, :, :3] *= alpha
        premul = Image.fromarray(np.clip(arr, 0, 255).astype(np.uint8), "RGBA")
        premul = premul.resize((size, size), Image.LANCZOS)
        # Unpremultiply
        arr2 = np.array(premul, dtype=np.float32)
        alpha2 = arr2[:, :, 3:4] / 255.0
        mask = alpha2 > 0.004  # ~1/255
        arr2[:, :, :3] = np.where(mask, arr2[:, :, :3] / np.maximum(alpha2, 0.004), 0)
        img = Image.fromarray(np.clip(arr2, 0, 255).astype(np.uint8), "RGBA")
    img = _remove_small_clusters(img)
    if standardize:
        display_angle = trs_entry.get('display_angle', -45.0) if trs_entry else -45.0
        force_flip = bool(trs_entry.get('flip', False)) if trs_entry else False
        item_fill = trs_entry.get('fill_ratio', 0.70) if trs_entry else 0.70
        img = _standardize_item_image(img, size, target_angle_deg=display_angle,
                                       fill_ratio=item_fill, force_flip=force_flip)
    return img


def _rasterize_triangle(
    px, py, pz, uvs, vi, ti,
    tex, default_color, color_buf, z_buf, size,
):
    """Rasterize a single triangle with texture mapping."""
    idx = [vi[0], vi[2], vi[1]]

    nv = len(px)
    nuv = len(uvs)
    if any(i < 0 or i >= nv for i in idx):
        return

    x0, y0, z0 = px[idx[0]], py[idx[0]], pz[idx[0]]
    x1, y1, z1 = px[idx[1]], py[idx[1]], pz[idx[1]]
    x2, y2, z2 = px[idx[2]], py[idx[2]], pz[idx[2]]

    uv_idx = [ti[0], ti[2], ti[1]]
    has_uv = tex is not None and all(0 <= i < nuv for i in uv_idx)

    if has_uv:
        u0, v0_uv = uvs[uv_idx[0]]
        u1, v1_uv = uvs[uv_idx[1]]
        u2, v2_uv = uvs[uv_idx[2]]

    edge1 = np.array([x1 - x0, y1 - y0, z1 - z0])
    edge2 = np.array([x2 - x0, y2 - y0, z2 - z0])
    normal = np.cross(edge1, edge2)
    nl = np.linalg.norm(normal)
    if nl < 1e-8:
        return
    normal /= nl

    # Diffuse: Lambertian (abs for double-sided)
    ndl_main = abs(np.dot(normal, _LIGHT_DIR))
    ndl_rim = abs(np.dot(normal, _RIM_DIR))

    # Hemisphere fill: normals pointing down get extra light (prevents pure black)
    hemi = (1.0 - abs(normal[1])) * 0.5 + 0.5  # 0.5 for down-facing, 1.0 for up-facing
    hemi_light = hemi * _HEMI_INTENSITY

    # Specular: Blinn-Phong (half-vector)
    half_main = _LIGHT_DIR - _VIEW_DIR
    half_main = half_main / (np.linalg.norm(half_main) + 1e-8)
    spec = max(0.0, np.dot(normal, half_main)) ** _SPECULAR_POWER * _SPECULAR_INTENSITY

    shade = _AMBIENT + hemi_light + ndl_main * _DIRECT_INTENSITY + ndl_rim * _RIM_INTENSITY + spec

    min_x = max(0, int(min(x0, x1, x2)))
    max_x = min(size - 1, int(max(x0, x1, x2)) + 1)
    min_y = max(0, int(min(y0, y1, y2)))
    max_y = min(size - 1, int(max(y0, y1, y2)) + 1)

    if min_x >= max_x or min_y >= max_y:
        return

    det = (y1 - y2) * (x0 - x2) + (x2 - x1) * (y0 - y2)
    if abs(det) < 1e-8:
        return
    inv_det = 1.0 / det

    for sy in range(min_y, max_y + 1):
        for sx in range(min_x, max_x + 1):
            w0 = ((y1 - y2) * (sx - x2) + (x2 - x1) * (sy - y2)) * inv_det
            w1 = ((y2 - y0) * (sx - x2) + (x0 - x2) * (sy - y2)) * inv_det
            w2 = 1.0 - w0 - w1

            if w0 < -0.001 or w1 < -0.001 or w2 < -0.001:
                continue

            z = w0 * z0 + w1 * z1 + w2 * z2
            if z <= z_buf[sy, sx]:
                continue

            if has_uv:
                u = w0 * u0 + w1 * u1 + w2 * u2
                v = w0 * v0_uv + w1 * v1_uv + w2 * v2_uv
                r, g, b, a = _sample_texture(tex, u, v)
            else:
                r, g, b, a = default_color

            # Skip fully transparent texels (don't write z-buffer)
            if a < 8:
                continue
            z_buf[sy, sx] = z

            # sRGB decode → linear space for correct shading
            lr = (r / 255.0) ** _SRGB_GAMMA
            lg = (g / 255.0) ** _SRGB_GAMMA
            lb = (b / 255.0) ** _SRGB_GAMMA

            # Apply shading in linear space + ACES Filmic tone mapping
            sr = lr * shade * _TONEMAP_EXPOSURE
            sg = lg * shade * _TONEMAP_EXPOSURE
            sb = lb * shade * _TONEMAP_EXPOSURE
            # ACES Filmic: (x*(2.51x+0.03))/(x*(2.43x+0.59)+0.14)
            tr = (sr * (2.51 * sr + 0.03)) / (sr * (2.43 * sr + 0.59) + 0.14)
            tg = (sg * (2.51 * sg + 0.03)) / (sg * (2.43 * sg + 0.59) + 0.14)
            tb = (sb * (2.51 * sb + 0.03)) / (sb * (2.43 * sb + 0.59) + 0.14)
            # Linear → sRGB encode
            inv_gamma = 1.0 / _SRGB_GAMMA
            fr = tr ** inv_gamma
            fg = tg ** inv_gamma
            fb = tb ** inv_gamma
            color_buf[sy, sx] = (
                min(255, max(0, int(fr * 255 + 0.5))),
                min(255, max(0, int(fg * 255 + 0.5))),
                min(255, max(0, int(fb * 255 + 0.5))),
                a,
            )


# ─── Batch Processing ────────────────────────────────────────────────

def process_item(args: tuple) -> tuple[str, bool, str]:
    """Process a single item: parse BMD → render → save WebP."""
    section, index, name, model_file, output_path, trs_entry = args

    bmd_path = ITEM_DIR / model_file
    if not bmd_path.exists():
        return name, False, f"BMD not found: {model_file}"

    try:
        meshes, bones = parse_bmd(str(bmd_path))
        if not meshes:
            return name, False, "No meshes in BMD"

        img = render_bmd(meshes, RENDER_SIZE, trs_entry, bones)
        Path(output_path).parent.mkdir(parents=True, exist_ok=True)
        img.save(output_path, "WEBP", quality=WEBP_QUALITY)
        return name, True, ""
    except Exception as e:
        import traceback
        traceback.print_exc()
        return name, False, str(e)


def main():
    import argparse
    parser = argparse.ArgumentParser(description="MU Online BMD 3D Renderer")
    parser.add_argument("--test", type=int, default=0,
                        help="Render only first N items for testing")
    parser.add_argument("--section", type=int, default=None,
                        help="Render only items from this section number")
    parser.add_argument("--index", type=int, default=None,
                        help="Render only item with this index (requires --section)")
    cli_args = parser.parse_args()

    # Load ItemTRSData.bmd for per-item rotation/scale
    trs_data = load_trs_data()
    trs_count = len(trs_data)
    print(f"TRS data: {trs_count} items loaded" if trs_count else "TRS data: not found (using fallback)")

    tree = ET.parse(str(ITEMLIST_XML))
    root = tree.getroot()

    items = []
    for section in root.findall("Section"):
        sec_idx = int(section.get("Index"))
        sec_name = section.get("Name")
        if cli_args.section is not None and sec_idx != cli_args.section:
            continue
        for item in section.findall("Item"):
            if cli_args.index is not None and int(item.get("Index")) != cli_args.index:
                continue
            model_file = item.get("ModelFile", "")
            if not model_file:
                continue
            items.append({
                "section": sec_idx,
                "section_name": sec_name,
                "index": int(item.get("Index")),
                "name": item.get("Name"),
                "model_file": model_file,
            })

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    tasks = []
    for it in items:
        mf = it["model_file"]
        out_path = str(OUTPUT_DIR / str(it['section']) / f"{it['index']}.webp")
        trs_entry = trs_data.get((it["section"], it["index"]))
        tasks.append((it["section"], it["index"], it["name"], mf, out_path, trs_entry))

    if cli_args.test > 0:
        tasks = tasks[:cli_args.test]
        items = items[:cli_args.test]

    total = len(tasks)
    mode = ""
    if cli_args.section is not None:
        mode = f" (Section {cli_args.section})"
    elif cli_args.test > 0:
        mode = f" (TEST: first {cli_args.test})"
    print(f"MU Online BMD 3D Renderer → WebP{mode}")
    print(f"Items: {total}, Unique models: {len(set(t[3] for t in tasks))}")
    print(f"Output: {OUTPUT_DIR}")
    print("-" * 60)

    success = 0
    failed = 0
    errors = []
    start = time.time()

    # Process sequentially (texture cache + z-buffer not easily shared across processes)
    for i, args in enumerate(tasks, 1):
        name, ok, err = process_item(args)
        if ok:
            success += 1
        else:
            failed += 1
            errors.append((name, err))

        if i % 20 == 0 or i == total:
            elapsed = time.time() - start
            rate = i / elapsed if elapsed > 0 else 0
            print(f"  [{i}/{total}] {rate:.1f} items/sec | OK: {success} | Failed: {failed}")

    elapsed = time.time() - start
    print("-" * 60)
    print(f"Done in {elapsed:.1f}s")
    print(f"Rendered: {success}/{total}")

    if errors:
        print(f"\nFailed ({failed}):")
        for n, err in errors[:20]:
            print(f"  {n}: {err}")

    # Save manifest
    manifest = []
    for it in items:
        manifest.append({
            "section": it["section"],
            "section_name": it["section_name"],
            "index": it["index"],
            "name": it["name"],
            "model_file": it["model_file"],
            "image": f"{it['section']}/{it['index']}.webp",
        })

    manifest_path = OUTPUT_DIR / "manifest.json"
    with open(manifest_path, "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2, ensure_ascii=False)
    print(f"Manifest: {manifest_path}")

    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
