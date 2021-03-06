package filereceiver

import (
	"fmt"
	"io"
	"strings"

	"github.com/zengchen1024/obs-worker/utils"
)

type CPIOFileMeta struct {
	Name         string
	MD5          string
	OriginalName string

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
		buf, err := utils.ReadData(r.reader, 110)
		if err != nil {
			return nil, fmt.Errorf("read cpio file header, err: %s", err.Error())
		}

		header := &CPIOFileHeader{}
		if err := header.extract(buf); err != nil {
			return nil, fmt.Errorf("extract cpio file header, err: %s", err.Error())
		}

		// read file name
		buf, err = utils.ReadData(r.reader, header.GetNameStreamSize())
		if err != nil {
			return nil, fmt.Errorf("read cpio file name, err: %s", err.Error())
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
			err := utils.EmptyRead(r.reader, header.GetFileStreamSize())
			if err != nil {
				return nil, fmt.Errorf("empty read cpio file, err: %s", err.Error())
			}

			continue
		}
		meta.Name = name

		v, err := r.handleCPIOFile(header, saveTo, calcMD5)
		if err != nil {
			return nil, fmt.Errorf("handle cpio file, err: %s", err.Error())
		}
		meta.MD5 = v

		metas = append(metas, meta)
	}

	return metas, nil
}

func (r *cpioReceiver) handleCPIOFile(header *CPIOFileHeader, saveTo string, calcMD5 bool) (string, error) {
	if saveTo != "" {
		if err := r.saveCPIOFile(header, saveTo); err != nil {
			return "", err
		}

		if calcMD5 {
			return utils.GenMd5OfFile(saveTo)
		}

		return "", nil
	}

	v, err := utils.GenMd5OfByteStream(r.reader, header.Size)
	if err != nil {
		return v, err
	}

	n := header.GetPad()
	if n == 0 {
		return v, nil
	}

	_, err = utils.ReadData(r.reader, n)
	return v, err
}

func (r *cpioReceiver) saveCPIOFile(header *CPIOFileHeader, file string) error {
	err := utils.DownloadFileWithSize(r.reader, header.Size, file)
	if err != nil {
		return err
	}

	if n := header.GetPad(); n > 0 {
		_, err := utils.ReadData(r.reader, n)

		return err
	}

	return nil
}
