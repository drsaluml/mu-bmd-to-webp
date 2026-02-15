#!/usr/bin/env python3
"""
Decode ItemTRSData.bmd — XOR encrypted binary with per-item TRS transforms.

File structure:
  - 4 bytes: int32 LE item count
  - N * 32 bytes: XOR encrypted records
  - 4 bytes: footer

Each 32-byte record (after XOR decrypt):
  int32 itemID (section*512+index) + float32 posX + float32 posY + float32 posZ
  + float32 rotX + float32 rotY + float32 rotZ + float32 scale
"""
import struct
import sys
import xml.etree.ElementTree as ET
from xml.dom import minidom
from pathlib import Path

XOR_KEY = bytes([0xfc, 0xcf, 0xab])


def xor_decrypt(data: bytes) -> bytes:
    out = bytearray(len(data))
    for i in range(len(data)):
        out[i] = data[i] ^ XOR_KEY[i % 3]
    return bytes(out)


def main():
    filepath = Path(__file__).parent / "Data" / "Local" / "itemtrsdata.bmd"
    raw = filepath.read_bytes()

    count = struct.unpack_from("<I", raw, 0)[0]
    print(f"File: {filepath}")
    print(f"Size: {len(raw)} bytes | Count: {count} | Record: 32 bytes")
    print()

    records = []
    offset = 4
    for i in range(count):
        enc = raw[offset:offset + 32]
        dec = xor_decrypt(enc)

        item_id = struct.unpack_from("<I", dec, 0)[0]
        px, py, pz = struct.unpack_from("<fff", dec, 4)
        rx, ry, rz = struct.unpack_from("<fff", dec, 16)
        scale = struct.unpack_from("<f", dec, 28)[0]

        section = item_id // 512
        index = item_id % 512

        records.append({
            'item_id': item_id,
            'section': section,
            'index': index,
            'posX': px, 'posY': py, 'posZ': pz,
            'rotX': rx, 'rotY': ry, 'rotZ': rz,
            'scale': scale,
        })
        offset += 32

    # Print first 40 records
    print(f"{'#':>5} | {'ID':>5} {'sec':>3}:{' idx':<4} | {'posX':>8} {'posY':>8} {'posZ':>8} | {'rotX':>7} {'rotY':>7} {'rotZ':>7} | {'scale':>8}")
    print("-" * 100)
    for i, r in enumerate(records[:40]):
        print(f"{i:5} | {r['item_id']:5} {r['section']:3}:{r['index']:<4} | {r['posX']:8.4f} {r['posY']:8.4f} {r['posZ']:8.1f} | {r['rotX']:7.1f} {r['rotY']:7.1f} {r['rotZ']:7.1f} | {r['scale']:8.5f}")

    # Find section boundaries
    print()
    print("=== Section boundaries ===")
    prev_sec = -1
    for i, r in enumerate(records):
        if r['section'] != prev_sec:
            print(f"  Record #{i}: section {r['section']} starts (item_id={r['item_id']}, index={r['index']})")
            prev_sec = r['section']

    # Show items with non-default values (different from the most common)
    print()
    print("=== Non-default TRS items (first 20) ===")
    non_default = [r for r in records if r['posX'] != 0.8 or r['posY'] != 0.85 or r['rotY'] != 270.0 or r['rotZ'] != 15.0 or r['scale'] != 0.0025]
    for r in non_default[:20]:
        print(f"  sec={r['section']:2} idx={r['index']:3} (id={r['item_id']:5}): "
              f"pos=({r['posX']:.3f},{r['posY']:.3f},{r['posZ']:.0f}) "
              f"rot=({r['rotX']:.0f},{r['rotY']:.0f},{r['rotZ']:.0f}) "
              f"scale={r['scale']:.5f}")

    # Stats
    print()
    print(f"Total records: {len(records)}")
    print(f"Sections found: {sorted(set(r['section'] for r in records))}")
    print(f"Non-default items: {len(non_default)}")

    # Export to XML
    xml_root = ET.Element("ItemTRSData")
    xml_root.set("Count", str(count))

    current_section = None
    section_el = None
    for r in records:
        if r['section'] != current_section:
            current_section = r['section']
            section_el = ET.SubElement(xml_root, "Section")
            section_el.set("Index", str(current_section))

        item_el = ET.SubElement(section_el, "Item")
        item_el.set("Index", str(r['index']))
        item_el.set("ItemID", str(r['item_id']))
        item_el.set("PosX", f"{r['posX']:.4f}")
        item_el.set("PosY", f"{r['posY']:.4f}")
        item_el.set("PosZ", f"{r['posZ']:.1f}")
        item_el.set("RotX", f"{r['rotX']:.1f}")
        item_el.set("RotY", f"{r['rotY']:.1f}")
        item_el.set("RotZ", f"{r['rotZ']:.1f}")
        item_el.set("Scale", f"{r['scale']:.6f}")

    xml_str = minidom.parseString(ET.tostring(xml_root, encoding="unicode")).toprettyxml(indent="  ")
    xml_path = Path(__file__).parent / "ItemTRSData.xml"
    xml_path.write_text(xml_str, encoding="utf-8")
    print(f"\nXML exported: {xml_path} ({xml_path.stat().st_size} bytes)")


if __name__ == "__main__":
    main()
