package utils

import (
	"crypto/md5"
	"fmt"
	"os"

	"github.com/beego/beego/v2/core/logs"
)

func GenMD5(b []byte) string {
	return fmt.Sprintf("%x", md5.Sum(b))
}

func GenMd5OfFile(f string) (string, error) {
	v, err := os.ReadFile(f)
	if err != nil {
		return "", err
	}

	return GenMD5(v), nil
}

func LogInfo(f string, v ...interface{}) {
	logs.Info(fmt.Sprintf(f, v...))
}

func LogErr(f string, v ...interface{}) {
	logs.Error(fmt.Sprintf(f, v...))
}
