package ctlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/pikvm/kvmd-cloud/internal/config"
)

func newUnixClient() http.Client {
	unixFilename := config.Cfg.UnixCtlSocket
	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", unixFilename)
			},
		},
	}
	return httpc
}

func DoUnixRequest(ctx context.Context, method string, url string, body io.Reader) (*http.Response, error) {
	client := newUnixClient()
	req, err := http.NewRequestWithContext(ctx, method, "http://unix"+config.Cfg.UnixCtlSocket+url, body)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	return resp, err
}

func DoUnixRequestJSON(ctx context.Context, method string, url string, body interface{}, data interface{}) error {
	var outBody io.Reader = nil
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return err
		}
		outBody = bytes.NewReader(bodyBytes)
	}
	resp, err := DoUnixRequest(ctx, method, url, outBody)
	if err != nil {
		return err
	}
	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(responseBytes, &data)
}
