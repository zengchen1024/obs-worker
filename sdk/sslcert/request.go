package sslcert

import (
	"io"
	"net/http"

	"github.com/zengchen1024/obs-worker/utils"
)

func List(hc *utils.HttpClient, endpoint, project string, autoExtend bool) (string, error) {
	p := map[string]string{
		"project": project,
	}
	if autoExtend {
		p["autoextend"] = "1"
	}

	url, err := utils.GenQueryURI(endpoint+"/getsslcert", p)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	var cert string
	var err1 error
	handle := func(resp io.Reader) {
		v, err := io.ReadAll(resp)
		if err != nil {
			err1 = err
		} else {
			cert = string(v)
		}
	}

	if err = hc.ForwardTo(req, handle); err != nil {
		return "", err
	}

	return cert, err1
}
