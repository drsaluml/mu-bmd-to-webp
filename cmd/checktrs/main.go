package main

import (
	"fmt"
	"os"
	"strconv"

	"mu-bmd-renderer/internal/trs"
)

func main() {
	sec := 12
	maxIdx := 30
	if len(os.Args) > 1 {
		sec, _ = strconv.Atoi(os.Args[1])
	}
	if len(os.Args) > 2 {
		maxIdx, _ = strconv.Atoi(os.Args[2])
	}

	data, err := trs.Load("Data/Local/itemtrsdata.bmd", "custom_trs.json", "Data/Xml/ItemList.xml")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for idx := 0; idx <= maxIdx; idx++ {
		key := [2]int{sec, idx}
		e, ok := data[key]
		if !ok {
			fmt.Printf("%d_%d: NO TRS\n", sec, idx)
			continue
		}
		bStr := "nil"
		if e.UseBones != nil {
			bStr = fmt.Sprintf("%v", *e.UseBones)
		}
		sStr := "nil"
		if e.Standardize != nil {
			sStr = fmt.Sprintf("%v", *e.Standardize)
		}
		fmt.Printf("%d_%d: src=%-7s rot=(%.1f,%.1f,%.1f) sc=%.4f bones=%s std=%s DA=%.1f cam=%q\n",
			sec, idx, e.Source, e.RotX, e.RotY, e.RotZ, e.Scale, bStr, sStr, e.DisplayAngle, e.Camera)
	}
}
