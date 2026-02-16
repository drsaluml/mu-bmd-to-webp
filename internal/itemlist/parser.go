package itemlist

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
)

// xmlItemList matches the ItemList.xml schema.
type xmlItemList struct {
	Sections []xmlSection `xml:"Section"`
}

type xmlSection struct {
	Index string    `xml:"Index,attr"`
	Name  string    `xml:"Name,attr"`
	Items []xmlItem `xml:"Item"`
}

type xmlItem struct {
	Index     string `xml:"Index,attr"`
	Name      string `xml:"Name,attr"`
	ModelFile string `xml:"ModelFile,attr"`
}

// Parse reads ItemList.xml and returns all items with model files.
func Parse(xmlPath string) ([]ItemDef, error) {
	raw, err := os.ReadFile(xmlPath)
	if err != nil {
		return nil, fmt.Errorf("itemlist: read %s: %w", xmlPath, err)
	}

	var list xmlItemList
	if err := xml.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("itemlist: parse %s: %w", xmlPath, err)
	}

	var items []ItemDef
	for _, sec := range list.Sections {
		secIdx, err := strconv.Atoi(sec.Index)
		if err != nil {
			continue
		}
		for _, item := range sec.Items {
			if item.ModelFile == "" {
				continue
			}
			idx, err := strconv.Atoi(item.Index)
			if err != nil {
				continue
			}
			items = append(items, ItemDef{
				Section:     secIdx,
				SectionName: sec.Name,
				Index:       idx,
				Name:        item.Name,
				ModelFile:   item.ModelFile,
			})
		}
	}

	return items, nil
}
