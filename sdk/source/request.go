package source

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/zengchen1024/obs-worker/sdk/filereceiver"
	"github.com/zengchen1024/obs-worker/utils"
)

type ListOpts struct {
	Project string `json:"project,omitempty"`
	Package string `json:"package,omitempty"`
	Srcmd5  string `json:"srcmd5,omitempty"`
}

func (l *ListOpts) toMap() (map[string]string, error) {
	b, err := json.Marshal(l)
	if err != nil {
		return nil, err
	}

	v := make(map[string]string)
	err = json.Unmarshal(b, &v)
	return v, err
}

func List(hc *utils.HttpClient, endpoint string, opts *ListOpts, check filereceiver.CPIOPreCheck) (meta []filereceiver.CPIOFileMeta, err error) {
	p, err := opts.toMap()
	if err != nil {
		return nil, err
	}

	url, err := utils.GenQueryURI(endpoint+"/getsources", p)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	handle := func(h http.Header, resp io.Reader) error {
		meta, err = filereceiver.ReceiveCpioFiles(resp, check)

		return err
	}

	err = hc.ForwardTo(req, handle)

	return
}
