package utils

import (
	"bufio"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

var ErrCancel = fmt.Errorf("canceled")

func GenMD5(b []byte) string {
	return fmt.Sprintf("%x", md5.Sum(b))
}

func GenMd5OfFile(f string) (string, error) {
	fi, err := os.Open(f)
	if err != nil {
		return "", err
	}

	defer fi.Close()

	h := md5.New()

	if err := transfer(fi, h); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func GenMd5OfByteStream(r io.Reader, size int64) (string, error) {
	h := md5.New()
	lr := &limitedRead{size, r}

	if err := transfer(lr, h); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func LogInfo(f string, v ...interface{}) {
	logrus.Infof(f, v...)
}

func LogErr(f string, v ...interface{}) {
	logrus.Errorf(f, v...)
}

func ReadFileLineByLine(filename string, handle func(string) bool) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if b := handle(scanner.Text()); b {
			break
		}
	}

	return nil
}

func WriteFile(f string, data []byte) error {
	return os.WriteFile(f, data, 0644)
}

func ReadData(r io.Reader, total int) ([]byte, error) {
	buf := make([]byte, total)

	n, err := ReadTo(nil, r, buf)
	if err != nil {
		return nil, err
	}

	if n != total {
		return buf[:n], io.EOF
	}

	return buf, nil
}

func ReadTo(ctx context.Context, r io.Reader, buf []byte) (int, error) {
	last := len(buf)

	for start, n := 0, 0; last > 0; {
		if ctx != nil && IsCtxDone(ctx) {
			return 0, ErrCancel
		}

		if n = 8192; last < n {
			n = last
		}

		n, err := r.Read(buf[start : start+n])
		if err != nil && n == 0 {
			if errors.Is(err, io.EOF) {
				return start, nil
			}

			return start, err
		}

		start += n
		last -= n
	}

	return len(buf), nil
}

func Write(ctx context.Context, w io.Writer, data []byte) error {
	for offset, total := 0, len(data); offset < total; {
		if ctx != nil && IsCtxDone(ctx) {
			return ErrCancel
		}

		n, err := w.Write(data[offset:])
		if err != nil {
			return err
		}

		offset += n
	}

	return nil
}
