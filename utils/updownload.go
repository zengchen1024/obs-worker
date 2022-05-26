package utils

import (
	"fmt"
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

	return limitedTransfer(r, size, fo)
}

func EmptyRead(r io.Reader, size int64) error {
	return DownloadFileWithSize(r, size, os.DevNull)
}

func limitedTransfer(r io.Reader, size int64, w io.Writer) error {
	read := func() ([]byte, error) {
		if size == 0 {
			return nil, nil
		}

		buf := make([]byte, 1<<13)

		if size < int64(len(buf)) {
			buf = buf[:size]
		}

		n, err := ReadTo(nil, r, buf)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, fmt.Errorf("oh no, left %d data to read", size)
		}

		size -= int64(n)

		return buf[:n], nil
	}

	write := func(data []byte) error {
		return Write(nil, w, data)
	}

	return TransferData(read, write)
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
