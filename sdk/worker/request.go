package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/zengchen1024/obs-worker/utils"
)

type QueryOpts struct {
	WorkerId string `json:"workerid,omitempty"`
	State    string `json:"state" required:"true"`
	Arch     string `json:"arch" required:"true"`
	Port     int    `json:"-"`
}

func (o *QueryOpts) toQuery() (string, error) {
	if o.Port == 0 {
		return "", fmt.Errorf("missing port")
	}

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

	q.Add("port", strconv.Itoa(o.Port))

	return q.Encode(), nil
}

func Create(endpoint string, opts *QueryOpts, w *Worker) (err error) {
	q, err := opts.toQuery()
	if err != nil {
		return
	}

	urlStr, err := utils.GenURL(endpoint+"/worker", q)
	if err != nil {
		return
	}

	data, err := w.Marshal()
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(data))
	if err != nil {
		return
	}

	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	return utils.ForwardTo(req, nil)
}

func Get(endpoint string, opts *QueryOpts) (err error) {
	q, err := opts.toQuery()
	if err != nil {
		return
	}

	urlStr, err := utils.GenURL(endpoint+"/worker", q)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return
	}

	return utils.ForwardTo(req, nil)
}
