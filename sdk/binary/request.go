package binary

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opensourceways/obs-worker/sdk/filereceiver"
	"github.com/opensourceways/obs-worker/utils"
)

type ListOpts struct {
	Project    string   `json:"project,omitempty"`
	Repository string   `json:"repository,omitempty"`
	Arch       string   `json:"arch,omitempty"`
	NoMeta     bool     `json:"-"`
	Binaries   []string `json:"-"`
	Modules    []string `json:"-"`
}

func (o *ListOpts) toQuery() (string, error) {
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

	if o.NoMeta {
		q.Add("nometa", "1")
	}

	if len(o.Binaries) > 0 {
		q.Add("binaries", strings.Join(o.Binaries, ","))
	}

	for _, m := range o.Modules {
		q.Add("module", m)
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
	WorkerId   string   `json:"workerid,omitempty"`
	Project    string   `json:"project" required:"true"`
	Repository string   `json:"repository" required:"true"`
	Arch       string   `json:"arch" required:"true"`
	Binaries   []string `json:"-"`
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

	q.Add("metaonly", "1")

	if len(o.Binaries) > 0 {
		q.Add("binaries", strings.Join(o.Binaries, ","))
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
