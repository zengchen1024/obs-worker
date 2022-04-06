package obsbuild

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func genmd5(b []byte) string {
	return fmt.Sprintf("%x", md5.Sum(b))
}

type cpioFileMeta struct {
	name         string
	originalName string

	md5  string
	data string

	cpioHeader
}

type cpioHeader struct {
	namesize int
	size     int64
	mtime    int64
	mode     int64
	cpiotype int
	pad      int
	namepad  int
}

type cpioReceiver struct {
	reader io.Reader
	// return 1. new name, 2. path to save file, 3. whether calc md5
	precheck func(string, *cpioHeader) (string, string, bool, error)
}

func (r *cpioReceiver) do() ([]cpioFileMeta, error) {
	metas := []cpioFileMeta{}

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
		buf = make([]byte, header.namesize+header.namepad)
		_, err = r.read("name", buf, true)
		if err != nil {
			return nil, err
		}

		name := string(buf[:header.namesize])
		name = strings.Trim(name, "\x00")
		if header.size == 0 && name == "TRAILER!!!" {
			break
		}
		name = strings.TrimPrefix(name, "./")

		if name == "." || name == ".." {
			return nil, fmt.Errorf("cpio filename is %s", name)
		}

		meta := cpioFileMeta{
			originalName: name,
			cpioHeader:   header,
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
		meta.name = name

		// read file
		file, err := r.readFile(name, &header)
		if err != nil {
			return nil, err
		}

		if saveTo != "" {
			err = os.WriteFile(saveTo, file, 0644)
			if err != nil {
				return nil, err
			}
		} else {
			meta.data = string(file)
		}

		if calcMD5 {
			meta.md5 = genmd5(file)
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

func (r *cpioReceiver) readFile(name string, header *cpioHeader) ([]byte, error) {
	last := header.size + int64(header.pad)
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

	if header.pad > 0 {
		return buf[:header.size], nil
	}

	return buf, nil
}

func (r *cpioReceiver) parseHeader(bs []byte) (cpioHeader, error) {
	n := len(bs)
	if n < 110 || string(bs[:6]) != "070701" {
		return cpioHeader{}, fmt.Errorf("not cpio file")
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
			return cpioHeader{}, fmt.Errorf("invalid size")
		}
	}

	namesize := hex(substr(94, 8))
	if namesize > 8192 {
		return cpioHeader{}, fmt.Errorf("ridiculous long filename")
	}

	h := cpioHeader{
		mode:     hex(substr(14, 8)),
		mtime:    hex(substr(46, 8)),
		namesize: int(namesize),
		size:     size,
		pad:      pad,
	}

	h.namepad = (6 - (h.namesize % 4)) % 4
	h.cpiotype = int(h.mode >> 12 & 0xf)

	return h, nil
}
