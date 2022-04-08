package source

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/opensourceways/obs-worker/utils"
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

// return 1. new name, 2. path to save file, 3. whether calc md5
type CPIOPreCheck func(string, *CPIOFileHeader) (string, string, bool, error)

func List(hc *utils.HttpClient, endpoint string, opts *ListOpts, check CPIOPreCheck) ([]CPIOFileMeta, error) {
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

	var meta []CPIOFileMeta
	var err1 error
	handle := func(resp io.Reader) {
		r := cpioReceiver{resp, check}
		meta, err1 = r.do()
	}

	if err = hc.ForwardTo(req, handle); err != nil {
		return nil, err
	}

	return meta, err1
}
