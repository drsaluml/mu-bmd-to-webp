package itemlist

// ItemDef holds one item parsed from ItemList.xml.
type ItemDef struct {
	Section     int
	SectionName string
	Index       int
	Name        string
	ModelFile   string // e.g. "sword04.bmd"
}
