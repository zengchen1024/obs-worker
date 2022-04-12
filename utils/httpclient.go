package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type HttpClient struct {
	cli        http.Client
	MaxRetries int
}

func (hc *HttpClient) ForwardTo(req *http.Request, handle func(http.Header, io.Reader) error) error {
	resp, err := hc.do(req)
	if err != nil || resp == nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		rb, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("response has status:%s and body:%q", resp.Status, rb)
	}

	if handle != nil {
		return handle(resp.Header, resp.Body)
	}

	return nil
}

func (hc *HttpClient) do(req *http.Request) (resp *http.Response, err error) {
	if resp, err = hc.cli.Do(req); err == nil {
		return
	}

	maxRetries := hc.MaxRetries
	backoff := 10 * time.Millisecond

	for retries := 1; retries < maxRetries; retries++ {
		time.Sleep(backoff)
		backoff *= 2

		if resp, err = hc.cli.Do(req); err == nil {
			break
		}
	}
	return
}

func ReadOnce(r io.Reader, part string, buf []byte, checkLen bool) (int, error) {
	n, err := r.Read(buf)
	if err != nil && n == 0 {
		return n, fmt.Errorf("read %s, err: %v", part, err)
	}

	if checkLen && n != len(buf) {
		return n, fmt.Errorf(
			"encounter unexpect EOF for %s, expect to read %d bytes, but got %d",
			part, len(buf), n,
		)
	}

	return n, nil
}

func ReadData(r io.Reader, name string, total int64) ([]byte, error) {
	last := total
	buf := make([]byte, last)

	var pn int64
	var start int64
	for start = 0; last > 0; start += pn {
		pn = 8192
		if last < pn {
			pn = last
		}
		last -= pn

		_, err := ReadOnce(r, name, buf[start:start+pn], true)
		if err != nil {
			return nil, err
		}
	}

	return buf, nil
}

func JsonMarshal(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	enc := json.NewEncoder(buffer)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(t); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func GenQueryURI(endpoint string, params map[string]string) (string, error) {
	v, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	if len(params) > 0 {
		q := v.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		v.RawQuery = q.Encode()
	}

	return v.String(), nil
}

func GenURL(endpoint, query string) (string, error) {
	v, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	if query != "" {
		v.RawQuery = query
	}

	return v.String(), nil
}
