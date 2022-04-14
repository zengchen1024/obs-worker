package config

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/opensourceways/obs-worker/sdk/filereceiver"
	"github.com/opensourceways/obs-worker/utils"
)

type DownloadOpts struct {
	Project    string `json:"project" required:"true"`
	Repository string `json:"repository" required:"true"`
}

func (o *DownloadOpts) toQuery() (string, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}

	v := make(map[string]string)
	err = json.Unmarshal(b, &v)
	if err != nil {
		return "", err
	}

	q := make(url.Values)
	for k, item := range v {
		q.Add(k, item)
	}

	return q.Encode(), nil
}

func Download(hc *utils.HttpClient, endpoint string, opts *DownloadOpts, saveTo string) error {
	q, err := opts.toQuery()
	if err != nil {
		return err
	}

	urlStr, err := utils.GenURL(endpoint+"/getconfig", q)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return err
	}

	handle := func(h http.Header, r io.Reader) error {
		return filereceiver.ReceiveFile(h, r, saveTo)
	}

	return hc.ForwardTo(req, handle)
}
