package bmd

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"

	"mu-bmd-renderer/internal/crypto"
)

// Parse reads a BMD file and returns meshes and bones.
// Supports versions 10 (unencrypted), 12 (XOR), and 15 (LEA-256 ECB).
func Parse(filepath string) ([]Mesh, []Bone, error) {
	raw, err := os.ReadFile(filepath)
	if err != nil {
		return nil, nil, fmt.Errorf("bmd: read %s: %w", filepath, err)
	}

	if len(raw) < 4 || string(raw[:3]) != "BMD" {
		return nil, nil, fmt.Errorf("bmd: invalid header in %s", filepath)
	}

	version := raw[3]
	var data []byte

	switch version {
	case 15:
		if len(raw) < 8 {
			return nil, nil, fmt.Errorf("bmd: truncated v15 header in %s", filepath)
		}
		size := binary.LittleEndian.Uint32(raw[4:8])
		if 8+int(size) > len(raw) {
			return nil, nil, fmt.Errorf("bmd: truncated v15 data in %s", filepath)
		}
		data = crypto.DecryptLEA(raw[8:8+size], crypto.LEAKey)
	case 12:
		if len(raw) < 8 {
			return nil, nil, fmt.Errorf("bmd: truncated v12 header in %s", filepath)
		}
		size := binary.LittleEndian.Uint32(raw[4:8])
		if 8+int(size) > len(raw) {
			return nil, nil, fmt.Errorf("bmd: truncated v12 data in %s", filepath)
		}
		data = crypto.DecryptXOR(raw[8 : 8+size])
	default:
		data = raw[4:]
	}

	r := &reader{data: data}
	return r.parse(filepath)
}

type reader struct {
	data []byte
	off  int
}

func (r *reader) readStr(n int) string {
	if r.off+n > len(r.data) {
		r.off = len(r.data)
		return ""
	}
	s := r.data[r.off : r.off+n]
	r.off += n
	// Find null terminator
	for i, b := range s {
		if b == 0 {
			return string(s[:i])
		}
	}
	return string(s)
}

func (r *reader) readI16() int16 {
	if r.off+2 > len(r.data) {
		r.off = len(r.data)
		return 0
	}
	v := int16(binary.LittleEndian.Uint16(r.data[r.off:]))
	r.off += 2
	return v
}

func (r *reader) readU16() uint16 {
	if r.off+2 > len(r.data) {
		r.off = len(r.data)
		return 0
	}
	v := binary.LittleEndian.Uint16(r.data[r.off:])
	r.off += 2
	return v
}

func (r *reader) readF32() float32 {
	if r.off+4 > len(r.data) {
		r.off = len(r.data)
		return 0
	}
	v := math.Float32frombits(binary.LittleEndian.Uint32(r.data[r.off:]))
	r.off += 4
	return v
}

func (r *reader) readByte() byte {
	if r.off >= len(r.data) {
		return 0
	}
	b := r.data[r.off]
	r.off++
	return b
}

func (r *reader) parse(filepath string) ([]Mesh, []Bone, error) {
	_ = r.readStr(32) // model name
	meshCount := int(r.readU16())
	boneCount := int(r.readU16())
	actionCount := int(r.readU16())

	if meshCount > 100 || meshCount < 0 {
		return nil, nil, fmt.Errorf("bmd: invalid mesh count %d in %s", meshCount, filepath)
	}

	meshes := make([]Mesh, 0, meshCount)
	for i := 0; i < meshCount; i++ {
		nv := int(r.readI16())
		nn := int(r.readI16())
		ntc := int(r.readI16())
		nt := int(r.readI16())
		_ = r.readI16() // texture index

		// Vertices: 16 bytes each (node:i16, pad:i16, x:f32, y:f32, z:f32)
		verts := make([][3]float32, nv)
		nodes := make([]int16, nv)
		for j := 0; j < nv; j++ {
			nodes[j] = r.readI16()
			_ = r.readI16() // padding
			verts[j][0] = r.readF32()
			verts[j][1] = r.readF32()
			verts[j][2] = r.readF32()
		}

		// Normals: 20 bytes each (node:i16, pad:i16, nx:f32, ny:f32, nz:f32, bind:i16, pad:i16)
		normals := make([][3]float32, nn)
		for j := 0; j < nn; j++ {
			_ = r.readI16() // node
			_ = r.readI16() // padding
			normals[j][0] = r.readF32()
			normals[j][1] = r.readF32()
			normals[j][2] = r.readF32()
			_ = r.readI16() // bindVertex
			_ = r.readI16() // padding
		}

		// TexCoords: 8 bytes each (u:f32, v:f32)
		uvs := make([][2]float32, ntc)
		for j := 0; j < ntc; j++ {
			uvs[j][0] = r.readF32()
			uvs[j][1] = r.readF32()
		}

		// Triangles: 64 bytes each
		tris := make([]Triangle, nt)
		for j := 0; j < nt; j++ {
			base := r.off
			if base+64 > len(r.data) {
				break
			}
			poly := int(r.data[base])
			var vi, ni, ti [4]int16
			for k := 0; k < 4; k++ {
				vi[k] = int16(binary.LittleEndian.Uint16(r.data[base+2+k*2:]))
			}
			for k := 0; k < 4; k++ {
				ni[k] = int16(binary.LittleEndian.Uint16(r.data[base+10+k*2:]))
			}
			for k := 0; k < 4; k++ {
				ti[k] = int16(binary.LittleEndian.Uint16(r.data[base+18+k*2:]))
			}
			tris[j] = Triangle{Polygon: poly, VI: vi, NI: ni, TI: ti}
			r.off += 64
		}

		texPath := r.readStr(32)
		// Normalize backslashes
		texPath = strings.ReplaceAll(texPath, "\\", "/")

		meshes = append(meshes, Mesh{
			Verts:   verts,
			Nodes:   nodes,
			Normals: normals,
			UVs:     uvs,
			Tris:    tris,
			TexPath: texPath,
		})
	}

	// Parse actions
	actionKeys := make([]int, actionCount)
	for a := 0; a < int(actionCount); a++ {
		numKeys := int(r.readI16())
		lockPos := r.readByte() > 0
		if lockPos {
			r.off += numKeys * 12 // skip float32 x,y,z per key
		}
		actionKeys[a] = numKeys
	}

	// Parse bones
	bones := make([]Bone, 0, boneCount)
	for b := 0; b < int(boneCount); b++ {
		isDummy := r.readByte() > 0
		if isDummy {
			bones = append(bones, Bone{Parent: -1, IsDummy: true})
			continue
		}

		_ = r.readStr(32) // bone name
		parent := int(r.readI16())

		var bindPos, bindRot [3]float64
		for a := 0; a < int(actionCount); a++ {
			numKeys := 0
			if a < len(actionKeys) {
				numKeys = actionKeys[a]
			}
			if numKeys == 0 {
				continue
			}
			// Positions: numKeys × (x, y, z) float32
			for k := 0; k < numKeys; k++ {
				px := float64(r.readF32())
				py := float64(r.readF32())
				pz := float64(r.readF32())
				if a == 0 && k == 0 {
					bindPos = [3]float64{px, py, pz}
				}
			}
			// Rotations: numKeys × (rx, ry, rz) float32
			for k := 0; k < numKeys; k++ {
				rx := float64(r.readF32())
				ry := float64(r.readF32())
				rz := float64(r.readF32())
				if a == 0 && k == 0 {
					bindRot = [3]float64{rx, ry, rz}
				}
			}
		}

		bones = append(bones, Bone{
			Parent:       parent,
			IsDummy:      false,
			BindPosition: bindPos,
			BindRotation: bindRot,
		})
	}

	return meshes, bones, nil
}
