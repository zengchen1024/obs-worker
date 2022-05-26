package controllers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/directory"
	"github.com/zengchen1024/obs-worker/utils"
	"github.com/zengchen1024/obs-worker/worker"
)

func (b BuildController) GetBuildLog(w http.ResponseWriter, r *http.Request) {
	callback := func(file string) error {
		q := r.URL.Query()

		if q.Get("view") == "entry" {
			return b.loginfo(file, w)
		}

		start := 0
		if v := q.Get("start"); v != "" {
			i, err := strconv.Atoi(v)
			if err != nil {
				return err
			}

			start = i
		}

		var end *int64
		if v := q.Get("end"); v != "" {
			i, err := strconv.Atoi(v)
			if err != nil {
				return err
			}

			j := int64(i)
			end = &j
		}

		return b.uploadLog(file, int64(start), end, w)
	}

	err := worker.GetBuildManager().GetBuildLog(b.jobid(r), callback)
	if err != nil {
		b.replyMsg(w, 400, err.Error())
	}
}

func (b BuildController) loginfo(file string, w http.ResponseWriter) error {
	info, err := os.Stat(file)
	if err != nil {
		return err
	}

	dir := directory.Directory{
		Entries: []directory.Entry{
			{
				Name:  "_log",
				Size:  info.Size(),
				Mtime: info.ModTime().Unix(),
			},
		},
	}

	b.reply(w, 0, &dir)

	return nil
}

func (b BuildController) uploadLog(file string, start int64, end *int64, w http.ResponseWriter) error {
	total := int64(0)
	if end != nil {
		if total = *end - start; total <= 0 {
			return fmt.Errorf("end is smaller than start")
		}
	}

	info, err := os.Stat(file)
	if err != nil {
		return err
	}

	v := start
	if v < 0 {
		v = -v
	}
	if info.Size() < v {
		return fmt.Errorf("log file is not that big")
	}

	f, err := os.Open(file)
	if err != nil {
		return err
	}

	defer f.Close()

	whence := os.SEEK_SET
	if start < 0 {
		whence = os.SEEK_END
	}

	pos, err := f.Seek(start, whence)
	if err != nil {
		return err
	}

	w.WriteHeader(200)

	header := []string{
		"HTTP/1.1 200 OK",
		"Content-Type: text/plain",
		"Transfer-Encoding: chunked",
		"Cache-Control: no-cache",
		"Connection: close",
		separator,
	}
	fmt.Fprint(w, strings.Join(header, separator))

	if v := info.Size() - pos; total == 0 || total > v {
		total = v
	}

	upload(f, total, w)

	return nil
}

func upload(r io.Reader, total int64, w io.Writer) {
	read := func() ([]byte, error) {
		if total == 0 {
			return nil, nil
		}

		n := 4096
		buf := make([]byte, n+8)

		if total < int64(n) {
			n = int(total)
		}

		n, err := utils.ReadTo(nil, r, buf[6:n+6])
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, fmt.Errorf("oh no, left %d data to read", total)
		}

		total -= int64(n)

		copy(buf, fmt.Sprintf("%04X\r\n", n))
		copy(buf[n+6:], []byte("\r\n"))

		return buf[:n+8], nil
	}

	write := func(data []byte) error {
		return utils.Write(nil, w, data)
	}

	if err := utils.TransferData(read, write); err != nil {
		return
	}

	fmt.Fprint(w, "0\r\n\r\n")
}
