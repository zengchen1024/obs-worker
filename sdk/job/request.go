package job

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/zengchen1024/obs-worker/sdk/filereceiver"
	"github.com/zengchen1024/obs-worker/utils"
)

type Opts struct {
	WorkerId string `json:"workerid,omitempty"`
	Job      string `json:"job" required:"true"`
	Arch     string `json:"arch" required:"true"`
	JobId    string `json:"jobid" required:"true"`
	Code     string `json:"code" required:"true"`
	KiwiTree bool   `json:"-"`
}

func (o *Opts) toQuery() (string, error) {
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

	if o.KiwiTree {
		q.Add("kiwitree", "1")
	}

	q.Add("now", strconv.Itoa(int(time.Now().Unix())))

	return q.Encode(), nil
}

type File struct {
	Name string
	Path string
}

func Put(hc *utils.HttpClient, endpoint string, opts Opts, files []File) (err error) {
	s, err := opts.toQuery()
	if err != nil {
		return
	}

	urlStr, err := utils.GenURL(endpoint+"/putjob", s)
	if err != nil {
		return
	}

	return upload(hc, urlStr, files)
}

func upload(hc *utils.HttpClient, urlStr string, files []File) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, w := io.Pipe()

	defer r.Close()

	go func() {
		// must close here, otherwise the http request will be blocked.
		// because the read will be done when the writer is done.
		// when the read is done, then the http request can continue.
		defer w.Close()

		write(ctx, w, files)
	}()

	req, err := http.NewRequest(http.MethodPost, urlStr, r)
	if err != nil {
		return err
	}

	req.Header.Set("Transfer-Encoding", "chunked")
	req.Header.Set("Content-Type", "application/x-cpio")

	return hc.ForwardTo(req, nil)
}

func write(ctx context.Context, w io.Writer, files []File) {
	for _, item := range files {
		if err := writeCpioFile(ctx, w, item); err != nil {
			utils.LogErr("write %s failed, err:%s", item.Path, err.Error())

			return
		}
	}

	s := "07070100000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000b00000000TRAILER!!!\x00\x00\x00\x000\r\n\r\n"

	if err := utils.WriteChunk(ctx, w, []byte(s)); err != nil {
		utils.LogErr("write the last chunk failed, err:%s", err.Error())
	}
}

func writeCpioFile(ctx context.Context, w io.Writer, file File) error {
	info, err := os.Stat(file.Path)
	if err != nil {
		return err
	}

	h := filereceiver.Encode(info, file.Name, nil)

	size := 1 << 13
	buf := make([]byte, size)

	copy(buf, []byte(h))

	fi, err := os.Open(file.Path)
	if err != nil {
		return err
	}

	n, err := utils.ReadTo(ctx, fi, buf[len(h):])
	if err != nil {
		return err
	}

	if err := utils.WriteChunk(ctx, w, buf[:len(h)+n]); err != nil {
		return err
	}

	for {
		n, err := utils.ReadTo(ctx, fi, buf)
		if err != nil {
			return err
		}

		if n == 0 {
			break
		}

		if err := utils.WriteChunk(ctx, w, buf[:n]); err != nil {
			return err
		}

		if n < size {
			break
		}
	}

	return nil
}
