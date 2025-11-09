package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

/* ===== Offsets ===== */

// ETM (offsets to u32 pointers *inside the file*)
const (
	ETM_SELECTIONS_OFFSET      = 0x0A
	ETM_SELECTIONS_TEXT_OFFSET = 0x0E
	ETM_ID_LIST                = 0x12
)

// XBTF (offsets relative to trainerData base)
const (
	XBTF_SELECTIONS_OFFSET      = 0x0A
	XBTF_SELECTIONS_TEXT_OFFSET = 0x0E
	XBTF_ID_LIST                = 0x12
)

/* ===== Model ===== */

type Trainer struct {
	IsXBTF      bool
	Raw         []byte
	TrainerData []byte

	CreationKey string
	Scroller    string
	Name        string
	Labels      []string
	TitleIDs    [3]uint32

	TextStart uint32
	OptStart  uint32
}

/* ===== Flags (UTF + rounded by default) ===== */

var (
	flagPretty = flag.Bool("pretty", true, "render boxed pretty output")
	flagUTF    = flag.Bool("utf", true, "use Unicode box characters")
	flagColor  = flag.Bool("color", false, "enable ANSI color")
	flagWidth  = flag.Int("width", 100, "content width (60–200)")
	flagStyle  = flag.String("style", "rounded", "box style: rounded|square")
)

/* ===== Main ===== */

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatalf("usage: %s [flags] <trainer.etm|trainer.xbtf>", os.Args[0])
	}
	path := flag.Arg(0)

	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	var tr *Trainer
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".xbtf" {
		tr, err = parseXBTF(raw)
	} else {
		tr, err = parseETM(raw)
	}
	if err != nil {
		log.Fatal(err)
	}

	printTrainer(tr)
}

/* ===== Pretty Printer (no terminal deps) ===== */

type boxChars struct{ tl, tr, bl, br, h, v, tSepL, tSepR string }

// square (┌┐└┘)
var utfSquare = boxChars{"┌", "┐", "└", "┘", "─", "│", "├", "┤"}

// rounded (╭╮╰╯)
var utfRounded = boxChars{"╭", "╮", "╰", "╯", "─", "│", "├", "┤"}

// ASCII fallback
var asciiBox = boxChars{"+", "+", "+", "+", "-", "|", "+", "+"}

type palette struct{ dim, bold, cyan, yellow, reset string }

var noColor = palette{"", "", "", "", ""}

var ansi = palette{
	dim:    "\x1b[2m",
	bold:   "\x1b[1m",
	cyan:   "\x1b[36m",
	yellow: "\x1b[33m",
	reset:  "\x1b[0m",
}

type PrettyOpts struct {
	UTFBorders bool
	Color      bool
	Width      int
	Style      string // "rounded" | "square"
}

func printTrainer(t *Trainer) {
	if *flagPretty {
		PrintTrainerPretty(t, PrettyOpts{
			UTFBorders: *flagUTF,
			Color:      *flagColor,
			Width:      *flagWidth,
			Style:      strings.ToLower(*flagStyle),
		})
		return
	}

	// Plain legacy text (fallback)
	fmt.Printf("Format:     %s\n", ternary(t.IsXBTF, "XBTF", "ETM"))
	if t.Name != "" {
		fmt.Printf("Name:       %s\n", t.Name)
	}
	if t.CreationKey != "" {
		fmt.Printf("CreationKey:%s\n", t.CreationKey)
	}
	if t.Scroller != "" {
		fmt.Printf("Scroller:   %s\n", t.Scroller)
	}
	if t.TitleIDs != [3]uint32{} {
		fmt.Printf("TitleIDs:   %08X %08X %08X\n", t.TitleIDs[0], t.TitleIDs[1], t.TitleIDs[2])
	}
	if t.OptStart != 0 || t.TextStart != 0 {
		fmt.Printf("Offsets:    Opt=0x%X  Text=0x%X\n", t.OptStart, t.TextStart)
	}
	fmt.Printf("Options (%d):\n", len(t.Labels))
	for i, lbl := range t.Labels {
		if lbl == "" {
			continue
		}
		fmt.Printf("  %2d) %s\n", i+1, lbl)
	}
}

func PrintTrainerPretty(t *Trainer, opts PrettyOpts) {
	opts.Width = clamp(opts.Width, 60, 200)

	// choose box set
	var box boxChars
	if opts.UTFBorders {
		if opts.Style == "square" {
			box = utfSquare
		} else {
			box = utfRounded
		}
	} else {
		box = asciiBox
	}

	pal := noColor
	if opts.Color {
		pal = ansi
	}

	// helpers
	line := func(left, mid, right string, fill int) string {
		return left + strings.Repeat(mid, fill) + right + "\n"
	}
	padRight := func(s string, width int) string {
		n := width - utf8.RuneCountInString(s)
		if n <= 0 {
			return ""
		}
		return strings.Repeat(" ", n)
	}
	kv := func(key, val string) {
		k := key
		if k != "" {
			k = pal.dim + key + pal.reset + ": "
		}
		for _, w := range wrapWords(k+val, opts.Width) {
			fmt.Printf("%s %s%s\n", box.v, w, padRight(w, opts.Width))
		}
	}
	block := func(text string) {
		for _, w := range wrapWords(text, opts.Width) {
			fmt.Printf("%s %s%s\n", box.v, w, padRight(w, opts.Width))
		}
	}
	sectionRule := func() {
		fmt.Print(line(box.tSepL, box.h, box.tSepR, opts.Width+2))
	}

	// header (center title)
	title := t.Name
	if title == "" {
		title = "Trainer"
	}
	title = " " + title + " "
	leftPad := 1
	rightPad := opts.Width + 2 - leftPad - utf8.RuneCountInString(title)
	if rightPad < 0 {
		rightPad = 0
	}
	fmt.Printf("%s%s%s%s%s\n",
		box.tl,
		strings.Repeat(box.h, leftPad),
		title,
		strings.Repeat(box.h, rightPad),
		box.tr,
	)

	// meta
	kv("Format", ternary(t.IsXBTF, "XBTF", "ETM"))
	if t.CreationKey != "" {
		kv("CreationKey", t.CreationKey)
	}
	if t.TitleIDs != [3]uint32{} {
		kv("TitleIDs", fmt.Sprintf("%08X %08X %08X", t.TitleIDs[0], t.TitleIDs[1], t.TitleIDs[2]))
	}
	if t.OptStart != 0 || t.TextStart != 0 {
		kv("Offsets", fmt.Sprintf("Opt=0x%X  Text=0x%X", t.OptStart, t.TextStart))
	}

	// scroller
	sectionRule()
	kv("Scroller", "")
	if t.Scroller != "" {
		block(t.Scroller)
	} else {
		block("(none)")
	}

	// options
	sectionRule()
	kv(fmt.Sprintf("Options (%d)", len(t.Labels)), "")
	for i, opt := range t.Labels {
		if opt == "" {
			continue
		}
		prefix := fmt.Sprintf("%2d) ", i+1)
		for _, w := range wrapWords(prefix+opt, opts.Width) {
			fmt.Printf("%s %s%s\n", box.v, w, padRight(w, opts.Width))
		}
	}

	// footer
	fmt.Print(line(box.bl, box.h, box.br, opts.Width+2))
}

func wrapWords(s string, width int) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var out []string
	for _, para := range strings.Split(s, "\n") {
		para = strings.TrimRight(para, " \t")
		if para == "" {
			out = append(out, "")
			continue
		}
		words := strings.Fields(para)
		line := ""
		for _, w := range words {
			if line == "" {
				line = w
				continue
			}
			if utf8.RuneCountInString(line)+1+utf8.RuneCountInString(w) <= width {
				line += " " + w
				continue
			}
			out = append(out, line)
			line = w
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

/* ===== Parsing ===== */

func parseETM(raw []byte) (*Trainer, error) {
	if len(raw) < ETM_SELECTIONS_TEXT_OFFSET+4 {
		return nil, errors.New("ETM too small")
	}
	textStart := le32(raw, ETM_SELECTIONS_TEXT_OFFSET)
	if int(textStart) >= len(raw) {
		return nil, errors.New("ETM textStart OOB")
	}
	optStart := le32(raw, ETM_SELECTIONS_OFFSET)

	vec := readTextVector(raw, textStart, 128)
	for i := range vec {
		vec[i] = sanitize(vec[i])
	}

	t := &Trainer{
		IsXBTF:      false,
		Raw:         raw,
		TrainerData: raw,
		TextStart:   textStart,
		OptStart:    optStart,
	}
	if len(vec) > 0 {
		t.Name = vec[0]
	}
	if len(vec) > 1 {
		t.Scroller = vec[1]
	}
	if len(vec) > 2 {
		t.Labels = vec[2:]
	}
	t.TitleIDs = readTitleIDs(raw, ETM_ID_LIST)
	return t, nil
}

func parseXBTF(raw []byte) (*Trainer, error) {
	// Try in this order (unmangled/key path → unmangled/detect → raw/key path → raw/detect)
	if tr, err := parseXBTFMode(raw, true, true); err == nil {
		return tr, nil
	}
	if tr, err := parseXBTFMode(raw, true, false); err == nil {
		return tr, nil
	}
	if tr, err := parseXBTFMode(raw, false, true); err == nil {
		return tr, nil
	}
	if tr, err := parseXBTFMode(raw, false, false); err == nil {
		return tr, nil
	}
	return nil, errors.New("XBTF: could not locate payload/creation key")
}

func parseXBTFMode(raw []byte, doUnmangle bool, useKeyPath bool) (*Trainer, error) {
	buf := append([]byte(nil), raw...)
	if doUnmangle {
		if err := unmangleInPlace(buf); err != nil {
			return nil, err
		}
	}

	tdStart := -1
	ck, sc := "", ""

	if useKeyPath {
		// +4: creation key (cstring), then scroller (cstring), then payload
		key, n := cstring(buf, 4)
		if n == 0 {
			return nil, errors.New("no creation key at +4")
		}
		ck = sanitize(key)
		off := 4 + n
		s2, n2 := cstring(buf, off)
		if n2 == 0 {
			return nil, errors.New("no scroller after creation key")
		}
		sc = sanitize(s2)
		tdStart = off + n2
		if tdStart >= len(buf) {
			return nil, errors.New("no payload after scroller")
		}
	} else {
		base := detectTrainerBaseByStructure(buf, 16384)
		if base < 0 {
			return nil, errors.New("no plausible payload base found")
		}
		tdStart = base
	}

	td := buf[tdStart:]
	if len(td) < XBTF_SELECTIONS_TEXT_OFFSET+4 {
		return nil, errors.New("payload too small")
	}
	textStart := le32(td, XBTF_SELECTIONS_TEXT_OFFSET)
	if int(textStart) >= len(td) {
		return nil, errors.New("textStart OOB")
	}
	optStart := le32(td, XBTF_SELECTIONS_OFFSET)

	vec := readTextVector(td, textStart, 128)
	for i := range vec {
		vec[i] = sanitize(vec[i])
	}

	t := &Trainer{
		IsXBTF:      true,
		Raw:         raw,
		TrainerData: td,
		CreationKey: ck,
		Scroller:    sc,
		TextStart:   textStart,
		OptStart:    optStart,
	}
	if len(vec) > 0 {
		t.Name = vec[0]
	}
	if len(vec) > 1 && t.Scroller == "" {
		t.Scroller = vec[1]
	}
	if len(vec) > 2 {
		t.Labels = vec[2:]
	}
	t.TitleIDs = readTitleIDs(td, XBTF_ID_LIST)
	return t, nil
}

/* ===== Helpers ===== */

func le32(b []byte, off int) uint32 {
	return binary.LittleEndian.Uint32(b[off : off+4])
}

func cstring(b []byte, off int) (string, int) {
	if off < 0 || off >= len(b) {
		return "", 0
	}
	end := bytes.IndexByte(b[off:], 0x00)
	if end < 0 {
		return "", 0
	}
	return string(b[off : off+end]), end + 1
}

func readTextVector(td []byte, textStart uint32, capCount int) []string {
	var out []string
	for i := 0; i < capCount; i++ {
		ptrOff := int(textStart) + 4*i
		if ptrOff+4 > len(td) {
			break
		}
		ptr := int(le32(td, ptrOff))
		if ptr == 0 || ptr >= len(td) {
			break
		}
		s, n := cstring(td, ptr) // ptr is RELATIVE to td
		if n == 0 {
			break
		}
		out = append(out, s)
	}
	return out
}

func readTitleIDs(td []byte, idPtrOff int) [3]uint32 {
	var ids [3]uint32
	if idPtrOff < 0 || idPtrOff+4 > len(td) {
		return ids
	}
	listPtr := int(le32(td, idPtrOff)) // pointer RELATIVE to current base
	if listPtr < 0 || listPtr+12 > len(td) {
		return ids
	}
	ids[0] = le32(td, listPtr)
	ids[1] = le32(td, listPtr+4)
	ids[2] = le32(td, listPtr+8)
	return ids
}

/* ===== Unmangler (exact XBMC semantics) ===== */

func unmangleInPlace(buf []byte) error {
	if len(buf) < 0x38 || len(buf) < 4 {
		return io.ErrShortBuffer
	}
	eax := uint32(buf[0x27]) + uint32(buf[0x2F]) + uint32(buf[0x37])
	eax = uint32(uint64(eax) * 0x00FFFFFF & 0xFFFFFFFF)

	first := binary.LittleEndian.Uint32(buf[0:4]) ^ eax
	binary.LittleEndian.PutUint32(buf[0:4], first)

	ebx := first
	eax = 0
	total := len(buf) - 4 // ECX initial
	for i := 0; i < total; i++ {
		idx := 4 + i
		b := buf[idx]
		b ^= byte(ebx)                       // xor with low(ebx)
		b = byte(uint32(b-byte(eax)) & 0xFF) // sub low(eax)
		buf[idx] = b
		currECX := uint32(total - i) // ECX before LOOP dec
		eax = (eax + 3 + currECX) & 0xFFFFFFFF
	}
	return nil
}

/* ===== Base detection for XBTF when no key/scroller ===== */

func detectTrainerBaseByStructure(buf []byte, scanLimit int) int {
	limit := scanLimit
	if limit > len(buf) {
		limit = len(buf)
	}
	for base := 0; base < limit; base += 4 {
		if base+XBTF_SELECTIONS_TEXT_OFFSET+4 > len(buf) {
			break
		}
		opt := int(le32(buf, base+XBTF_SELECTIONS_OFFSET))
		txt := int(le32(buf, base+XBTF_SELECTIONS_TEXT_OFFSET))
		if opt == 0 || txt == 0 || opt >= txt {
			continue
		}
		if base+txt+4 > len(buf) {
			continue
		}
		firstPtr := int(le32(buf, base+txt))
		if firstPtr == 0 || base+firstPtr >= len(buf) {
			continue
		}
		if s, n := cstring(buf, base+firstPtr); n == 0 || s == "" {
			continue
		}
		return base
	}
	return -1
}

/* ===== String sanitation (ASCII only) ===== */

func sanitize(s string) string {
	if s == "" {
		return s
	}
	var b []rune
	for _, r := range s {
		// normalize whitespace
		if r == '\r' || r == '\t' || r == '\n' {
			r = ' '
		}
		// keep ASCII printable only
		if r < 32 || r > 126 {
			if r != ' ' {
				continue
			}
		}
		b = append(b, r)
	}
	// collapse spaces
	out := make([]rune, 0, len(b))
	space := false
	for _, r := range b {
		if r == ' ' {
			if space {
				continue
			}
			space = true
		} else {
			space = false
		}
		out = append(out, r)
	}
	return strings.TrimSpace(string(out))
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
