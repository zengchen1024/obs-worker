package filereceiver

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zengchen1024/obs-worker/utils"
)

type CPIOFileMeta struct {
	Name         string
	OriginalName string

	MD5 string

	CPIOFileHeader
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
		_, err := utils.ReadTo(nil, r.reader, buf)
		if err != nil {
			return nil, err
		}

		header := &CPIOFileHeader{}
		if err := header.extract(buf); err != nil {
			return nil, err
		}

		// reand file name
		buf = make([]byte, header.Namesize+header.Namepad)
		_, err = utils.ReadTo(nil, r.reader, buf)
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
			CPIOFileHeader: *header,
		}

		// pre check
		name, saveTo, calcMD5, err := r.precheck(name, header)
		if err != nil {
			return nil, err
		}

		if name == "" {
			_, err := utils.ReadData(r.reader, header.Size+int64(header.Pad))
			if err != nil {
				return nil, err
			}

			continue
		}

		meta.Name = name

		// read file
		tmp := saveTo
		if saveTo == "" {
			// TODO gen tmp
			defer os.Remove(tmp)
		}

		if err := r.saveCPIOFile(header, tmp); err != nil {
			return nil, err
		}

		if calcMD5 {
			if v, err := utils.GenMd5OfFile(tmp); err == nil {
				meta.MD5 = v
			} else {
				return nil, err
			}
		}

		metas = append(metas, meta)
	}

	return metas, nil
}

func (r *cpioReceiver) saveCPIOFile(header *CPIOFileHeader, file string) error {
	lr := &limitedRead{
		max: int(header.Size),
		r:   r.reader,
	}

	if err := Save(lr, file); err != nil {
		return err
	}

	if header.Pad > 0 {
		_, err := utils.ReadData(r.reader, int64(header.Pad))
		return err
	}

	return nil
}
