package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

func ForwardTo(req *http.Request, handle func(http.Header, io.Reader) error) error {
	resp, err := sendReq(req)
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

func sendReq(req *http.Request) (resp *http.Response, err error) {
	if resp, err = http.DefaultClient.Do(req); err == nil {
		return
	}

	maxRetries := 3
	backoff := 10 * time.Millisecond

	for retries := 1; retries < maxRetries; retries++ {
		time.Sleep(backoff)
		backoff *= 2

		if resp, err = http.DefaultClient.Do(req); err == nil {
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

func IsCtxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
