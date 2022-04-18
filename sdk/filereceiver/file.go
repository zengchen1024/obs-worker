package filereceiver

import (
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/zengchen1024/obs-worker/utils"
)

func ReceiveFile(h http.Header, r io.Reader, saveTo string) error {
	n, err := strconv.Atoi(h.Get("content-length"))
	if err != nil {
		return err
	}

	b, err := utils.ReadData(r, "", int64(n))
	if err != nil {
		return err
	}

	return os.WriteFile(saveTo, b, os.FileMode(0644))
}
