package utils

import (
	"io"
	"os"
)

func DownloadFile(r io.Reader, file string) error {
	fo, err := os.Create(file)
	if err != nil {
		return err
	}

	defer fo.Close()

	return transfer(r, fo)
}

func DownloadFileWithSize(r io.Reader, size int64, file string) error {
	fo, err := os.Create(file)
	if err != nil {
		return err
	}

	defer fo.Close()

	lr := &limitedRead{size, r}
	return transfer(lr, fo)
}

func EmptyRead(r io.Reader, size int64) error {
	return DownloadFileWithSize(r, size, os.DevNull)
}

func transfer(r io.Reader, w io.Writer) error {
	read := func() ([]byte, error) {
		buf := make([]byte, 1<<13)

		n, err := ReadTo(nil, r, buf)
		if err != nil || n == 0 {
			return nil, err
		}

		return buf[:n], nil
	}

	write := func(data []byte) error {
		return Write(nil, w, data)
	}

	return TransferData(read, write)
}

type limitedRead struct {
	max int64
	r   io.Reader
}

func (l *limitedRead) Read(buf []byte) (int, error) {
	if l.max == 0 {
		return 0, io.EOF
	}

	n, err := l.r.Read(buf)

	l.max -= int64(n)

	return n, err
}

func NewLimitedReader(max int64, r io.Reader) io.Reader {
	return &limitedRead{max, r}
}
