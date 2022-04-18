package binary

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/filereceiver"
	"github.com/zengchen1024/obs-worker/utils"
)

type CommonOpts struct {
	WorkerId   string   `json:"workerid,omitempty"`
	Project    string   `json:"project" required:"true"`
	Repository string   `json:"repository" required:"true"`
	Arch       string   `json:"arch" required:"true"`
	Modules    []string `json:"-"`
	Binaries   []string `json:"-"`
}

func (o *CommonOpts) values() (url.Values, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}

	v := make(map[string]string)
	err = json.Unmarshal(b, &v)
	if err != nil {
		return nil, err
	}

	q := make(url.Values)
	for k, item := range v {
		q.Add(k, item)
	}

	if len(o.Binaries) > 0 {
		q.Add("binaries", strings.Join(o.Binaries, ","))
	}

	for _, m := range o.Modules {
		q.Add("module", m)
	}

	return q, nil
}

type ListOpts struct {
	CommonOpts

	NoMeta bool `json:"-"`
}

func (o *ListOpts) toQuery() (string, error) {
	q, err := o.values()
	if err != nil {
		return "", err
	}

	if o.NoMeta {
		q.Add("nometa", "1")
	}

	return q.Encode(), nil
}

func List(hc *utils.HttpClient, endpoint string, opts *ListOpts) (binaries BinaryVersionList, err error) {
	q, err := opts.toQuery()
	if err != nil {
		return
	}

	urlStr, err := utils.GenURL(endpoint+"/getbinaryversions", q)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return
	}

	handle := func(h http.Header, r io.Reader) error {
		n, err := strconv.Atoi(h.Get("content-length"))
		if err != nil {
			return err
		}

		b, err := utils.ReadData(r, "", int64(n))
		if err != nil {
			return err
		}

		binaries, err = extract(b)

		return err
	}

	err = hc.ForwardTo(req, handle)

	return
}

type DownloadOpts struct {
	CommonOpts

	MetaOnly bool `json:"-"`
}

func (o *DownloadOpts) toQuery() (string, error) {
	q, err := o.values()
	if err != nil {
		return "", err
	}

	if o.MetaOnly {
		q.Add("metaonly", "1")
	}

	return q.Encode(), nil
}

func Download(hc *utils.HttpClient, endpoint string, opts *DownloadOpts, saveToDir string) (meta []filereceiver.CPIOFileMeta, err error) {
	q, err := opts.toQuery()
	if err != nil {
		return
	}

	urlStr, err := utils.GenURL(endpoint+"/getbinaries", q)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return
	}

	handle := func(h http.Header, r io.Reader) error {
		check := func(
			name string,
			header *filereceiver.CPIOFileHeader,
		) (string, string, bool, error) {
			return name, filepath.Join(saveToDir, name), false, nil
		}

		meta, err = filereceiver.ReceiveCpioFiles(r, check)

		return err
	}

	err = hc.ForwardTo(req, handle)

	return
}
