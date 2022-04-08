package utils

import (
	"crypto/md5"
	"fmt"
)

func GenMD5(b []byte) string {
	return fmt.Sprintf("%x", md5.Sum(b))
}
