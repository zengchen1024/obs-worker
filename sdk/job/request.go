package job

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
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

func Put(endpoint string, opts Opts, files []File) (err error) {
	s, err := opts.toQuery()
	if err != nil {
		return
	}

	urlStr, err := utils.GenURL(endpoint+"/putjob", s)
	if err != nil {
		return
	}

	return upload(urlStr, files)
}

func upload(urlStr string, files []File) error {
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

	return utils.ForwardTo(req, nil)
}

func write(ctx context.Context, w io.Writer, files []File) {
	r := newReader()

	for _, file := range files {
		if err := r.readFile(ctx, w, file); err != nil {
			utils.LogErr(
				"write failed cpio file%s, err:%s",
				file.Path, err.Error(),
			)

			return
		}
	}

	s := filereceiver.EncodeEmpty()

	if err := utils.Write(ctx, w, []byte(s)); err != nil {
		utils.LogErr("write the last chunk failed, err:%s", err.Error())
	}
}

func newReader() (r reader) {
	r.size = 1 << 13
	r.start = 0
	r.end = r.start + r.size

	// buf is chunked. the buf struct is: data + [\0 * pad]
	// the pad is 3 bytes at most, therefor the buf size if 8k + 4
	r.buf = make([]byte, r.size+4)

	return
}

type reader struct {
	buf   []byte
	size  int
	start int
	end   int
	pad   int
	total int64
}

func (r *reader) readFile(ctx context.Context, w io.Writer, file File) error {
	info, err := os.Stat(file.Path)
	if err != nil {
		return err
	}

	h, pad := filereceiver.Encode(info, file.Name, nil)

	r.pad = pad
	r.total = info.Size()

	fi, err := os.Open(file.Path)
	if err != nil {
		return err
	}

	defer fi.Close()

	if len(h) >= r.size {
		return fmt.Errorf("maybe too long file name")
	}

	copy(r.buf[r.start:], h)

	n, err := r.read(ctx, fi, r.start+len(h))
	if err != nil {
		return err
	}

	if err = utils.Write(ctx, w, r.buf[:n]); err != nil {
		return err
	}

	for r.total > 0 {
		n, err := r.read(ctx, fi, r.start)
		if err != nil {
			return err
		}

		if err = utils.Write(ctx, w, r.buf[:n]); err != nil {
			return err
		}
	}

	return nil
}

func (r *reader) read(ctx context.Context, fi io.Reader, start int) (int, error) {
	offset := start

	n, err := utils.ReadTo(ctx, fi, r.buf[offset:r.end])
	if err != nil {
		return 0, err
	}

	r.total -= int64(n)
	offset += n

	if r.total == 0 && r.pad > 0 {
		copy(
			r.buf[offset:],
			[]byte(strings.Repeat("\x00", r.pad)),
		)

		offset += r.pad
	}

	return offset, nil
}
