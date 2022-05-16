package filereceiver

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type CPIOFileHeader struct {
	Mtime    int64
	Mode     int64
	Size     int64
	Type     int
	Namesize int
	Namepad  int
	Pad      int
}

func (h *CPIOFileHeader) GetPad() int {
	return int(4-(h.Size&3)) & 3
}

func (h *CPIOFileHeader) getNamePad() int {
	return (6 - (h.Namesize & 3)) & 3
}

func (h *CPIOFileHeader) GetType() int {
	return int((h.Mode >> 12) & 0xf)
}

func (h *CPIOFileHeader) extract(bs []byte) error {
	if n := len(bs); n < 110 || string(bs[:6]) != "070701" {
		return fmt.Errorf("not cpio file")
	}

	substr := func(start, n int) string {
		return string(bs[start : start+n])
	}

	hex := func(s string) int64 {
		v, _ := strconv.ParseInt(s, 16, 0)
		return v
	}

	size := hex(substr(54, 8))
	if size == 0xffffffff {
		size = (hex(substr(78, 8)) << 32) + hex(substr(86, 8))
		if size < 0xffffffff {
			return fmt.Errorf("invalid size")
		}
	}

	namesize := hex(substr(94, 8))
	if namesize > 8192 {
		return fmt.Errorf("ridiculous long filename")
	}

	h.Mode = hex(substr(14, 8))
	h.Mtime = hex(substr(46, 8))
	h.Namesize = int(namesize)
	h.Size = size

	return nil
}

func (h *CPIOFileHeader) Marshal() string {
	s := fmt.Sprintf(
		"07070100000000%08x000000000000000000000001%08x",
		h.Mode, h.Mtime,
	)

	size := h.Size

	if size >= (0xffffffff) {
		top := size >> 32
		size = size & 0xffffffff
		s += fmt.Sprintf("ffffffff0000000000000000%08x%08x", top, size)
	} else {
		s += fmt.Sprintf("%08x00000000000000000000000000000000", h.Size)
	}

	return s + fmt.Sprintf("%08x00000000", h.Namesize)
}

func Encode(info os.FileInfo, name string, cpioType *uint) (string, int) {
	if info == nil {
		return "", 0
	}

	mode := uint(info.Mode())
	if mode == 0 {
		mode = 0x81a4
	}
	if cpioType != nil {
		mode = (mode & ^uint(0xf000)) | (*cpioType << 12)
	} else {
		if (mode & 0xf000) == 0 {
			mode |= 0x8000
		}
	}

	if name == "" {
		name = info.Name()
	}

	h := CPIOFileHeader{
		Mode:  int64(mode),
		Mtime: info.ModTime().Unix(),
		Size:  info.Size(),
	}

	return encode(&h, name), h.GetPad()
}

func EncodeEmpty() string {
	return encode(&CPIOFileHeader{}, "TRAILER!!!")
}

func encode(h *CPIOFileHeader, name string) string {
	name += "\x00"
	h.Namesize = len(name)

	s := h.Marshal() + name

	if n := len(s) & 3; n != 0 {
		s += strings.Repeat("\x00", 4-n)
	}

	return s
}
