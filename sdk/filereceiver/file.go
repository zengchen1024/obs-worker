package filereceiver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/zengchen1024/obs-worker/utils"
)

func ReceiveFile(h http.Header, r io.Reader, saveTo string) error {
	return Save(r, saveTo)
}

func Save(r io.Reader, file string) error {
	fo, err := os.Create(file)
	if err != nil {
		return err
	}

	defer fo.Close()

	h := downloadFileHelper{}
	return h.save(r, fo)
}

type downloadFileHelper struct {
	ch        chan []byte
	writeDone chan struct{}
}

func (h *downloadFileHelper) save(r io.Reader, w io.Writer) error {
	h.writeDone = make(chan struct{})
	h.ch = make(chan []byte, 1)

	wg := sync.WaitGroup{}

	var rerr error
	var werr error

	wg.Add(2)

	go func() {
		rerr = h.read(r)
		wg.Done()
	}()

	go func() {
		werr = h.write(w)
		wg.Done()
	}()

	wg.Wait()

	if rerr == nil && werr == nil {
		return nil
	}

	return fmt.Errorf("%v, %v", rerr, werr)
}

func (h *downloadFileHelper) read(r io.Reader) error {
	defer close(h.ch)

	for {
		buf := make([]byte, 1<<13)

		n, err := utils.ReadTo(nil, r, buf)
		if err != nil || n == 0 {
			return err
		}

		select {
		case <-h.writeDone:
			return nil
		case h.ch <- buf[:n]:
		}
	}
}

func (h *downloadFileHelper) write(w io.Writer) error {
	defer close(h.writeDone)

	for {
		data, ok := <-h.ch
		if !ok {
			return nil
		}

		if err := utils.Write(nil, w, data); err != nil {
			return err
		}
	}
}

type limitedRead struct {
	max int
	r   io.Reader
}

func (l *limitedRead) Read(buf []byte) (int, error) {
	if l.max == 0 {
		return 0, io.EOF
	}

	n, err := l.r.Read(buf)

	l.max -= n

	return n, err
}

func UploadLog(r io.Reader, w io.Writer, max int) error {
	lr := limitedRead{max, r}

	h := downloadFileHelper{}
	return h.save(&lr, w)
}
