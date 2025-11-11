package parser

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ETM (offsets to u32 pointers *inside the file*)
	ETM_SELECTIONS_OFFSET      = 0x0A
	ETM_SELECTIONS_TEXT_OFFSET = 0x0E
	ETM_ID_LIST                = 0x12

	// XBTF (offsets relative to trainerData base)
	XBTF_SELECTIONS_OFFSET      = 0x0A
	XBTF_SELECTIONS_TEXT_OFFSET = 0x0E
	XBTF_ID_LIST                = 0x12
)

type Trainer struct {
	IsXBTF      bool      `json:"isXBTF"`
	Path        string    `json:"path"`
	CreationKey string    `json:"creationKey"`
	Scroller    string    `json:"scroller"`
	Name        string    `json:"name"`
	Labels      []string  `json:"labels"`
	TitleIDs    [3]uint32 `json:"titleIDs"`
	TextStart   uint32    `json:"textStart"`
	OptStart    uint32    `json:"optStart"`
	IconURL     string    `json:"iconURL"`
}

func ParsePath(path string) (*Trainer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tr *Trainer
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xbtf":
		tr, err = parseXBTF(raw)
	case ".etm":
		tr, err = parseETM(raw)
	default:
		return nil, fmt.Errorf("unsupported file: %s", path)
	}
	if err != nil {
		return nil, err
	}
	tr.Path = path
	tr.IconURL = iconURLFor(tr)
	return tr, nil
}

func ParseDir(root string) ([]*Trainer, error) {
	var out []*Trainer
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".etm" && ext != ".xbtf" {
			return nil
		}
		t, e := ParsePath(p)
		if e != nil {
			// non-fatal: skip bad file
			return nil
		}
		out = append(out, t)
		return nil
	})
	return out, err
}

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
		IsXBTF:    false,
		TextStart: textStart,
		OptStart:  optStart,
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

func sanitize(s string) string {
	if s == "" {
		return s
	}
	var b []rune
	for _, r := range s {
		if r == '\r' || r == '\t' || r == '\n' {
			r = ' '
		}
		if r < 32 || r > 126 {
			if r != ' ' {
				continue
			}
		}
		b = append(b, r)
	}
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

func iconURLFor(t *Trainer) string {
	var tid uint32
	for _, v := range t.TitleIDs {
		if v != 0 {
			tid = v
			break
		}
	}
	if tid == 0 {
		return ""
	}
	id := fmt.Sprintf("%08X", tid)
	prefix := id[:4]
	return fmt.Sprintf("https://raw.githubusercontent.com/MobCat/MobCats-original-xbox-game-list/main/icon/%s/%s.png", prefix, id)
}
