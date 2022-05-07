package filereceiver

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/zengchen1024/obs-worker/utils"
)

type CPIOFileMeta struct {
	Name         string
	OriginalName string

	MD5  string
	Data string

	CPIOFileHeader
}

type CPIOFileHeader struct {
	Namesize int
	Size     int64
	Mtime    int64
	Mode     int64
	Type     int
	Pad      int
	Namepad  int
}

// return 1. new name, 2. path to save file, 3. whether calc md5
type CPIOPreCheck func(string, *CPIOFileHeader) (string, string, bool, error)

func ReceiveCpioFiles(resp io.Reader, check CPIOPreCheck) ([]CPIOFileMeta, error) {
	r := cpioReceiver{resp, check}
	return r.do()
}

type cpioReceiver struct {
	reader   io.Reader
	precheck CPIOPreCheck
}

func (r *cpioReceiver) do() ([]CPIOFileMeta, error) {
	metas := []CPIOFileMeta{}

	for {
		// read header
		buf := make([]byte, 110)
		_, err := r.read("header", buf, true)
		if err != nil {
			return nil, err
		}

		header, err := r.parseHeader(buf)
		if err != nil {
			return nil, err
		}

		// reand file name
		buf = make([]byte, header.Namesize+header.Namepad)
		_, err = r.read("name", buf, true)
		if err != nil {
			return nil, err
		}

		name := string(buf[:header.Namesize])
		name = strings.Trim(name, "\x00")
		if header.Size == 0 && name == "TRAILER!!!" {
			break
		}
		name = strings.TrimPrefix(name, "./")

		if name == "." || name == ".." {
			return nil, fmt.Errorf("cpio filename is %s", name)
		}

		meta := CPIOFileMeta{
			OriginalName:   name,
			CPIOFileHeader: header,
		}

		// pre check
		name, saveTo, calcMD5, err := r.precheck(name, &header)
		if err != nil {
			return nil, err
		}

		if name == "" {
			if _, err := r.readFile("", &header); err != nil {
				return nil, err
			}
			continue
		}
		meta.Name = name

		// read file
		file, err := r.readFile(name, &header)
		if err != nil {
			return nil, err
		}

		if saveTo != "" {
			err = utils.WriteFile(saveTo, file)
			if err != nil {
				return nil, err
			}
		} else {
			meta.Data = string(file)
		}

		if calcMD5 {
			meta.MD5 = utils.GenMD5(file)
		}

		metas = append(metas, meta)
	}

	return metas, nil
}

func (r *cpioReceiver) read(part string, buf []byte, checkLen bool) (int, error) {
	n, err := r.reader.Read(buf)
	if err != nil && n == 0 {
		return n, fmt.Errorf("read %s, err: %v", part, err)
	}

	if checkLen && n != len(buf) {
		return n, fmt.Errorf(
			"encounter unexpect EOF for %s, expect to read %d bytes, but got %d",
			part, len(buf), n,
		)
	}

	return n, nil
}

func (r *cpioReceiver) readFile(name string, header *CPIOFileHeader) ([]byte, error) {
	last := header.Size + int64(header.Pad)
	buf := make([]byte, last)

	var pn int64
	var start int64
	for start = 0; last > 0; start += pn {
		pn = 8192
		if last < pn {
			pn = last
		}
		last -= pn

		_, err := r.read("file: "+name, buf[start:start+pn], true)
		if err != nil {
			return nil, err
		}
	}

	if header.Pad > 0 {
		return buf[:header.Size], nil
	}

	return buf, nil
}

func (r *cpioReceiver) parseHeader(bs []byte) (CPIOFileHeader, error) {
	n := len(bs)
	if n < 110 || string(bs[:6]) != "070701" {
		return CPIOFileHeader{}, fmt.Errorf("not cpio file")
	}

	substr := func(start, n int) string {
		return string(bs[start : start+n])
	}

	hex := func(s string) int64 {
		v, _ := strconv.ParseInt(s, 16, 0)
		return v
	}

	size := hex(substr(54, 8))
	pad := int(4-(size%4)) % 4
	if size == 0xffffffff {
		size = hex(substr(86, 8))
		pad = int(4-(size%4)) % 4

		size += (hex(substr(78, 8)) << 32)
		if size < 0xffffffff {
			return CPIOFileHeader{}, fmt.Errorf("invalid size")
		}
	}

	namesize := hex(substr(94, 8))
	if namesize > 8192 {
		return CPIOFileHeader{}, fmt.Errorf("ridiculous long filename")
	}

	h := CPIOFileHeader{
		Mode:     hex(substr(14, 8)),
		Mtime:    hex(substr(46, 8)),
		Namesize: int(namesize),
		Size:     size,
		Pad:      pad,
	}

	h.Namepad = (6 - (h.Namesize % 4)) % 4
	h.Type = int(h.Mode >> 12 & 0xf)

	return h, nil
}
