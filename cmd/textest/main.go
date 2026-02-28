package main

import (
	"fmt"
	"mu-bmd-renderer/internal/trs"
)

func main() {
	data, _ := trs.Load("Data/Local/itemtrsdata.bmd", "custom_trs.json", "Data/Xml/ItemList.xml")
	e := data[[2]int{0, 114}]
	if e != nil {
		fmt.Printf("Section 0, Index 114:\n")
		fmt.Printf("  Source: %s\n", e.Source)
		fmt.Printf("  RotX=%.1f RotY=%.1f RotZ=%.1f Scale=%.5f\n", e.RotX, e.RotY, e.RotZ, e.Scale)
		fmt.Printf("  Camera: %q\n", e.Camera)
		if e.UseBones != nil {
			fmt.Printf("  UseBones: %v\n", *e.UseBones)
		} else {
			fmt.Printf("  UseBones: nil (auto)\n")
		}
		if e.Standardize != nil {
			fmt.Printf("  Standardize: %v\n", *e.Standardize)
		} else {
			fmt.Printf("  Standardize: nil (default)\n")
		}
	} else {
		fmt.Println("No TRS entry for section 0, index 114")
	}
}
