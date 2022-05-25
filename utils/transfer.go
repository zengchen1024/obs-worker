package utils

import (
	"fmt"
	"sync"
)

type TransferRead func() ([]byte, error)
type TransferWrite func([]byte) error

func TransferData(r TransferRead, w TransferWrite) error {
	t := transferData{}
	return t.do(r, w)
}

type transferData struct {
	ch        chan []byte
	writeDone chan struct{}
}

func (h *transferData) do(r TransferRead, w TransferWrite) error {
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

func (h *transferData) read(r TransferRead) error {
	defer close(h.ch)

	for {
		buf, err := r()
		if err != nil || len(buf) == 0 {
			return err
		}

		select {
		case <-h.writeDone:
			return nil
		case h.ch <- buf:
		}
	}
}

func (h *transferData) write(w TransferWrite) error {
	defer close(h.writeDone)

	for {
		data, ok := <-h.ch
		if !ok {
			return nil
		}

		if err := w(data); err != nil {
			return err
		}
	}
}
