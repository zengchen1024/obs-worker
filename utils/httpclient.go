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

func (hc *HttpClient) ForwardTo(req *http.Request, handle func(io.Reader)) error {
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
		handle(resp.Body)
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
