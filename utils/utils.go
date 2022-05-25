package utils

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

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

func GenMd5OfByteStream(r io.Reader, size int) (string, error) {
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
