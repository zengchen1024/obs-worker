package sslcert

import (
	"io"
	"net/http"

	"github.com/zengchen1024/obs-worker/utils"
)

func List(endpoint, project string, autoExtend bool) (
	cert []byte, err error,
) {
	p := map[string]string{
		"project": project,
	}
	if autoExtend {
		p["autoextend"] = "1"
	}

	url, err := utils.GenQueryURI(endpoint+"/getsslcert", p)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	handle := func(header http.Header, resp io.Reader) error {
		cert, err = io.ReadAll(resp)
		return err
	}

	err = utils.ForwardTo(req, handle)

	return
}
