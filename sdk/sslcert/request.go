package sslcert

import (
	"io"
	"net/http"

	"github.com/zengchen1024/obs-worker/sdk/filereceiver"
	"github.com/zengchen1024/obs-worker/utils"
)

func List(endpoint, project string, autoExtend bool, saveTo string) error {
	p := map[string]string{
		"project": project,
	}
	if autoExtend {
		p["autoextend"] = "1"
	}

	url, err := utils.GenQueryURI(endpoint+"/getsslcert", p)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	handle := func(header http.Header, resp io.Reader) error {
		return filereceiver.ReceiveFile(header, resp, saveTo)
	}

	return utils.ForwardTo(req, handle)
}
