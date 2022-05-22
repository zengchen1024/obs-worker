package image

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/zengchen1024/obs-worker/sdk/filereceiver"
	"github.com/zengchen1024/obs-worker/utils"
)

type QueryOpts struct {
	Prpa []string `json:"-"` // value is project/repository/arch
}

func (l *QueryOpts) toQuery() (string, error) {
	q := make(url.Values)

	q.Add("match", "body")

	for _, item := range l.Prpa {
		q.Add("prpa", item)
	}

	return q.Encode(), nil
}

func Post(hc *utils.HttpClient, endpoint string, opts *QueryOpts, data []byte, workDir string) (images []Image, err error) {
	q, err := opts.toQuery()
	if err != nil {
		return
	}

	urlStr, err := utils.GenURL(endpoint+"/getpreinstallimageinfos", q)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(data))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	handle := func(h http.Header, r io.Reader) error {
		n, err := strconv.Atoi(h.Get("content-length"))
		if err != nil {
			return err
		}

		b, err := utils.ReadData(r, "", int64(n))
		if err != nil {
			return err
		}

		images, err = extract(b, workDir)

		return err
	}

	err = hc.ForwardTo(req, handle)

	return
}

func Download(hc *utils.HttpClient, endpoint, prpa, path, saveTo string) error {
	urlStr, err := utils.GenURL(endpoint+fmt.Sprintf("/build/%s/%s", prpa, path), "")
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
