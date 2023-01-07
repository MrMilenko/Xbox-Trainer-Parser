/*
Ripped a lot of this from XBMC. I'm not even sure it's supposed to do what it's actually doing.
*/

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
	"log"
)

const (
	// header offsets etm
	etmSelectionsOffset = 0x0A // option states (0 or 1)
	etmSelectionsTextOffset = 0x0E // option labels
	etmIDList = 0x12 // TitleID(s) trainer is meant for
	etmEntryPoint = 0x16 // entry point for etm file (really just com file)

	// header offsets xbtf
	xbtfSelectionsOffset = 0x0A // option states (0 or 1)
	xbtfSelectionsTextOffset = 0x0E // option labels
	xbtfIDList = 0x12 // TitleID(s) trainer is meant for
	xbtfSection = 0x16 // section to patch the locations in memory our xbtf support functions end up
	xbtfEntryPoint = 0x1A // entry point for xbtf file (really com).
)

// Trainer represents a trainer file.
type Trainer struct {
	isXBTF       bool
	size         int
	data         []byte
	trainerData  []byte
	creationKey  string
	titleIDs     [3]uint32
	entryPoint   int
	options      []byte
	optionLabels []string
	section      int
}

// unmangleTrainer unmangles the given buffer of size bytes.
func unmangleTrainer(buffer unsafe.Pointer, size uint32) {
	// Load buffer and size into registers
	esi := uintptr(buffer)
	ecx := uint32(size)

	// Initialize registers
	eax, ebx := uint32(0), uint32(0)

	// Load values from buffer
	eax += uint32(*(*byte)(unsafe.Pointer(esi + 0x27)))
	eax += uint32(*(*byte)(unsafe.Pointer(esi + 0x2f)))
	eax += uint32(*(*byte)(unsafe.Pointer(esi + 0x37)))
	ecx = 0xffffff
	eax *= ecx
	*(*uint32)(unsafe.Pointer(buffer)) ^= eax
	ebx = *(*uint32)(unsafe.Pointer(buffer))
	esi += 4
	eax, ecx = uint32(0), size - 4

	// Loop
	for i := uint32(0); i < ecx; i++ {
		*(*byte)(unsafe.Pointer(esi)) ^= byte(ebx)
		*(*byte)(unsafe.Pointer(esi)) -= byte(eax)
		eax += 3
		eax += ecx
		esi++
	}
}

// Load reads a trainer file from the given path and returns a Trainer.
func (t *Trainer) Load(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if ext := strings.ToLower(filepath.Ext(path)); ext == ".xbtf" {
		t.isXBTF = true
	} else {
		t.isXBTF = false
	}

	info, err := file.Stat()
	if err != nil {
		return err
	}
	t.size = int(info.Size())
	if t.size < etmSelectionsOffset {
		return fmt.Errorf("broken trainer: %s", path)
	}

	t.data = make([]byte, t.size+1)
	t.data[t.size] = 0 // fix octal escape error
	_, err = file.Read(t.data)
	if err != nil {
		return err
	}

	if t.isXBTF {
		t.creationKey = string(t.data[4:204])
		if t.creationKey[6] != '-' {
			return fmt.Errorf("broken trainer: %s", path)
		}
		t.trainerData = t.data[204:]
		unmangleTrainer(unsafe.Pointer(&t.trainerData[0]), uint32(len(t.trainerData)))
		textLength := len(t.trainerData) - xbtfSelectionsTextOffset - xbtfSection
		t.titleIDs = [3]uint32{binary.LittleEndian.Uint32(t.trainerData[xbtfIDList : xbtfIDList+4])}
		t.section = int(t.trainerData[xbtfSection])
		t.entryPoint = int(t.trainerData[xbtfEntryPoint])
t.options = t.trainerData[xbtfSelectionsOffset : xbtfSelectionsOffset+2]
		t.optionLabels = strings.Split(string(t.trainerData[xbtfSelectionsTextOffset:xbtfSelectionsTextOffset+textLength]), "\x00")
	} else {
		t.titleIDs = [3]uint32{
			binary.LittleEndian.Uint32(t.data[etmIDList : etmIDList+4]),
			binary.LittleEndian.Uint32(t.data[etmIDList+4 : etmIDList+8]),
			binary.LittleEndian.Uint32(t.data[etmIDList+8 : etmIDList+12]),
		}
		t.entryPoint = int(t.data[etmEntryPoint])
		t.options = t.data[etmSelectionsOffset : etmSelectionsOffset+2]
		t.optionLabels = strings.Split(string(t.data[etmSelectionsTextOffset:]), "\x00")
	}
	return nil
}
// GetNumberOfOptions returns the number of options in the trainer.
func (t *Trainer) GetNumberOfOptions() int {
	return len(t.optionLabels)
}

// SetOptions updates the options in the trainer with the given data.
func (t *Trainer) SetOptions(options []byte) {
	t.options = options
}

// String returns a string representation of the trainer.
func (t *Trainer) String() string {
	return fmt.Sprintf("IsXBTF: %v\nSize: %d\nTitleIDs: %d, %d, %d\nEntryPoint: %d\nOptionLabels: %v\nOptions: %v\n", t.isXBTF, t.size, t.titleIDs[0], t.titleIDs[1], t.titleIDs[2], t.entryPoint, t.optionLabels, t.options)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "missing trainer path argument")
		os.Exit(1)
	}
	path := os.Args[1]

	trainer := &Trainer{}
	if err := trainer.Load(path); err != nil {
		log.Fatal(err)
	}
	fmt.Println(trainer)
}
