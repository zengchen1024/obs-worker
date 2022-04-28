package oldpkg

import (
	"fmt"
	"io"
	"net/http"

	"github.com/zengchen1024/obs-worker/sdk/filereceiver"
	"github.com/zengchen1024/obs-worker/utils"
)

type ListOpts struct {
	Project    string `json:"project,omitempty"`
	Repository string `json:"repository,omitempty"`
	Package    string `json:"package,omitempty"`
	Arch       string `json:"arch,omitempty"`
}

func List(
	hc *utils.HttpClient,
	endpoint string, opts *ListOpts,
	check filereceiver.CPIOPreCheck,
) (meta []filereceiver.CPIOFileMeta, err error) {
	p := map[string]string{
		"view":     "cpio",
		"noajax":   "1",
		"noimport": "1",
	}

	s := fmt.Sprintf(
		"%s/build/%s/%s/%s/%s",
		endpoint,
		opts.Project,
		opts.Repository,
		opts.Arch,
		opts.Package,
	)
	url, err := utils.GenQueryURI(s, p)
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
