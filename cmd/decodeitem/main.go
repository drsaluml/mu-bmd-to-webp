// cmd/decodeitem/main.go — Decode Data/Local/item.bmd → Data/Xml/ItemList.xml
//
// Usage:
//
//	go run ./cmd/decodeitem
//	go run ./cmd/decodeitem [input.bmd] [output.xml]
//
// Converts the encrypted item.bmd binary into the ItemList.xml format
// used by the renderer's config ("item_list_xml": "Data/Xml/ItemList.xml").
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

var xorKey = [3]byte{0xFC, 0xCF, 0xAB}

func xorDecrypt(data []byte) []byte {
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ xorKey[i%3]
	}
	return out
}

func readString(data []byte, offset, length int) string {
	if offset+length > len(data) {
		return ""
	}
	s := data[offset : offset+length]
	idx := 0
	for idx < len(s) && s[idx] != 0 {
		idx++
	}
	// Decode from Windows-1252 to UTF-8 (MU Online uses Windows encoding)
	decoded, err := charmap.Windows1252.NewDecoder().Bytes(s[:idx])
	if err != nil {
		return strings.TrimSpace(string(s[:idx]))
	}
	return strings.TrimSpace(string(decoded))
}

func u8(d []byte, off int) int  { return int(d[off]) }
func i8(d []byte, off int) int  { return int(int8(d[off])) }
func u16(d []byte, off int) int { return int(binary.LittleEndian.Uint16(d[off:])) }
func u32(d []byte, off int) int { return int(binary.LittleEndian.Uint32(d[off:])) }

type itemRecord struct {
	section int
	index   int

	name      string
	modelPath string
	modelFile string

	kindA, kindB, typ int
	slot, twoHand     int
	skillIndex        int
	width, height     int

	damageMin, damageMax     int
	defense                  int
	successfulBlocking       int
	attackSpeed, walkSpeed   int
	durability               int
	magicDurability          int
	magicPower               int
	dropLevel                int
	combatPower              int
	attackRate               int

	reqLevel, reqStrength, reqDexterity int
	reqEnergy, reqVitality, reqCommand  int

	money     int
	setAttrib int

	// 16 class flags
	darkWizard, darkKnight, fairyElf, magicGladiator int
	darkLord, summoner, rageFighter, growLancer       int
	runeWizard, slayer, gunCrusher, lightWizard       int
	lemuriaMage, illusionKnight, alchemist, crusader  int

	// 7 resistances
	iceRes, poisonRes, lightRes, fireRes int
	earthRes, windRes, waterRes          int

	// trade flags
	dump, transaction, personalStore int
	storeWarehouse, sellToNPC        int
	expensiveItem, repair            int
	overlap, nonValue                int

	elementalDefense int
}

var sectionNames = map[int]string{
	0: "Swords", 1: "Axes", 2: "Maces and Scepters", 3: "Spears",
	4: "Bows and Crossbows", 5: "Staffs", 6: "Shields",
	7: "Helmets", 8: "Armors", 9: "Pants", 10: "Gloves", 11: "Boots",
	12: "Pets and Rings and Misc", 13: "Jewel and Misc",
	14: "Wings and Orbs and Spheres", 15: "Scrolls", 16: "Muuns",
	19: "Uncategorized", 20: "Uncategorized", 21: "Cloaks",
}

// xmlEscape escapes special XML characters in attribute values.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func main() {
	inputPath := "Data/Local/item.bmd"
	outputPath := "Data/Xml/ItemList.xml"

	if len(os.Args) > 1 {
		inputPath = os.Args[1]
	}
	if len(os.Args) > 2 {
		outputPath = os.Args[2]
	}

	raw, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputPath, err)
		os.Exit(1)
	}

	if len(raw) < 8 {
		fmt.Fprintf(os.Stderr, "File too small\n")
		os.Exit(1)
	}

	itemCount := int(binary.LittleEndian.Uint32(raw[0:4]))
	bytesPerItem := (len(raw) - 8) / itemCount

	fmt.Fprintf(os.Stderr, "item.bmd: %d items, %d bytes/item, %d bytes total\n",
		itemCount, bytesPerItem, len(raw))

	// Group items by section
	sections := map[int][]itemRecord{}
	var sectionOrder []int

	offset := 4
	for i := 0; i < itemCount && offset+bytesPerItem <= len(raw)-4; i++ {
		rec := xorDecrypt(raw[offset : offset+bytesPerItem])

		group := u16(rec, 4)
		id := u16(rec, 6)

		modelFolder := readString(rec, 8, 260)
		modelName := readString(rec, 268, 260)
		itemName := readString(rec, 528, 64)

		item := itemRecord{
			section:   group,
			index:     id,
			name:      itemName,
			modelPath: modelFolder,
			modelFile: modelName,

			kindA:   u8(rec, 592),
			kindB:   u8(rec, 593),
			typ:     u8(rec, 594),
			twoHand: u8(rec, 595),

			dropLevel:  u16(rec, 596),
			slot:       i8(rec, 598),
			skillIndex: u16(rec, 600),
			width:      u8(rec, 602),
			height:     u8(rec, 603),

			damageMin:          u16(rec, 604),
			damageMax:          u16(rec, 606),
			successfulBlocking: u16(rec, 608),
			defense:            u16(rec, 610),
			walkSpeed:          u16(rec, 612),
			attackSpeed:        u16(rec, 614),
			durability:         u16(rec, 616),
			magicDurability:    u16(rec, 620),
			magicPower:         u32(rec, 624),
			combatPower:        u16(rec, 628),
			attackRate:         u16(rec, 632),

			reqStrength:  u16(rec, 636),
			reqDexterity: u16(rec, 638),
			reqEnergy:    u16(rec, 640),
			reqVitality:  u16(rec, 642),
			reqCommand:   u16(rec, 644),
			reqLevel:     u16(rec, 646),

			money:     u32(rec, 652),
			setAttrib: u8(rec, 656),

			darkWizard:     u8(rec, 657),
			darkKnight:     u8(rec, 658),
			fairyElf:       u8(rec, 659),
			magicGladiator: u8(rec, 660),
			darkLord:       u8(rec, 661),
			summoner:       u8(rec, 662),
			rageFighter:    u8(rec, 663),
			growLancer:      u8(rec, 664),
			runeWizard:     u8(rec, 665),
			slayer:         u8(rec, 666),
			gunCrusher:     u8(rec, 667),
			lightWizard:    u8(rec, 668),
			lemuriaMage:    u8(rec, 669),
			illusionKnight: u8(rec, 670),
			alchemist:      u8(rec, 671),
			crusader:       u8(rec, 672),

			iceRes:    u8(rec, 673),
			poisonRes: u8(rec, 674),
			lightRes:  u8(rec, 675),
			fireRes:   u8(rec, 676),
			earthRes:  u8(rec, 677),
			windRes:   u8(rec, 678),
			waterRes:  u8(rec, 679),

			dump:           u8(rec, 680),
			transaction:    u8(rec, 681),
			personalStore:  u8(rec, 682),
			storeWarehouse: u8(rec, 683),
			sellToNPC:      u8(rec, 684),
			expensiveItem:  u8(rec, 685),
			repair:         u8(rec, 686),
			overlap:        u8(rec, 687),
			nonValue:       u8(rec, 688),

			elementalDefense: u16(rec, 702),
		}

		if _, ok := sections[group]; !ok {
			sectionOrder = append(sectionOrder, group)
		}
		sections[group] = append(sections[group], item)

		offset += bytesPerItem
	}

	// Sort sections by index
	sort.Ints(sectionOrder)

	// Write XML in the exact ItemList.xml format
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<ItemList>\n")

	totalItems := 0
	for _, secIdx := range sectionOrder {
		items := sections[secIdx]
		name := sectionNames[secIdx]
		if name == "" {
			name = fmt.Sprintf("Section%d", secIdx)
		}

		sb.WriteString(fmt.Sprintf("\t<Section Index=\"%d\" Name=\"%s\">\n", secIdx, xmlEscape(name)))

		for _, it := range items {
			totalItems++
			sb.WriteString(fmt.Sprintf(
				"\t\t<Item Index=\"%d\" Name=\"%s\""+
					" KindA=\"%d\" KindB=\"%d\" Type=\"%d\""+
					" Slot=\"%d\" TwoHand=\"%d\" SkillIndex=\"%d\""+
					" Width=\"%d\" Height=\"%d\""+
					" DamageMin=\"%d\" DamageMax=\"%d\""+
					" Defense=\"%d\" SuccessfulBlocking=\"%d\""+
					" AttackSpeed=\"%d\" WalkSpeed=\"%d\""+
					" Durability=\"%d\" MagicDurability=\"%d\" MagicPower=\"%d\""+
					" DropLevel=\"%d\" CombatPower=\"%d\" AttackRate=\"%d\""+
					" ReqLevel=\"%d\" ReqStrength=\"%d\" ReqDexterity=\"%d\""+
					" ReqEnergy=\"%d\" ReqVitality=\"%d\" ReqCommand=\"%d\""+
					" Money=\"%d\" SetAttrib=\"%d\""+
					" DarkWizard=\"%d\" DarkKnight=\"%d\" FairyElf=\"%d\""+
					" MagicGladiator=\"%d\" DarkLord=\"%d\" Summoner=\"%d\""+
					" RageFighter=\"%d\" GrowLancer=\"%d\" RuneWizard=\"%d\""+
					" Slayer=\"%d\" GunCrusher=\"%d\" LightWizard=\"%d\""+
					" LemuriaMage=\"%d\" IllusionKnight=\"%d\""+
					" Alchemist=\"%d\" Crusader=\"%d\""+
					" IceRes=\"%d\" PoisonRes=\"%d\" LightRes=\"%d\""+
					" FireRes=\"%d\" EarthRes=\"%d\" WindRes=\"%d\" WaterRes=\"%d\""+
					" Dump=\"%d\" Transaction=\"%d\" PersonalStore=\"%d\""+
					" StoreWarehouse=\"%d\" SellToNPC=\"%d\""+
					" ExpensiveItem=\"%d\" Repair=\"%d\""+
					" Overlap=\"%d\" NonValue=\"%d\""+
					" ElementalDefense=\"%d\""+
					" ModelPath=\"%s\" ModelFile=\"%s\""+
					"></Item>\n",
				it.index, xmlEscape(it.name),
				it.kindA, it.kindB, it.typ,
				it.slot, it.twoHand, it.skillIndex,
				it.width, it.height,
				it.damageMin, it.damageMax,
				it.defense, it.successfulBlocking,
				it.attackSpeed, it.walkSpeed,
				it.durability, it.magicDurability, it.magicPower,
				it.dropLevel, it.combatPower, it.attackRate,
				it.reqLevel, it.reqStrength, it.reqDexterity,
				it.reqEnergy, it.reqVitality, it.reqCommand,
				it.money, it.setAttrib,
				it.darkWizard, it.darkKnight, it.fairyElf,
				it.magicGladiator, it.darkLord, it.summoner,
				it.rageFighter, it.growLancer, it.runeWizard,
				it.slayer, it.gunCrusher, it.lightWizard,
				it.lemuriaMage, it.illusionKnight,
				it.alchemist, it.crusader,
				it.iceRes, it.poisonRes, it.lightRes,
				it.fireRes, it.earthRes, it.windRes, it.waterRes,
				it.dump, it.transaction, it.personalStore,
				it.storeWarehouse, it.sellToNPC,
				it.expensiveItem, it.repair,
				it.overlap, it.nonValue,
				it.elementalDefense,
				xmlEscape(it.modelPath), xmlEscape(it.modelFile),
			))
		}

		sb.WriteString("\t</Section>\n")
	}

	sb.WriteString("</ItemList>\n")

	if err := os.WriteFile(outputPath, []byte(sb.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Decoded %d items in %d sections → %s\n",
		totalItems, len(sectionOrder), outputPath)
}
