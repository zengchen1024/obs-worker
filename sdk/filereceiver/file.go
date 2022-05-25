package filereceiver

import (
	"io"
	"net/http"

	"github.com/zengchen1024/obs-worker/utils"
)

func ReceiveFile(h http.Header, r io.Reader, saveTo string) error {
	return utils.DownloadFile(r, saveTo)
}
