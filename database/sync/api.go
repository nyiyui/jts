package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

type ServerClient struct {
	client  *http.Client
	baseURL *url.URL
}

func (sc *ServerClient) lock(ctx context.Context) error {
	url := sc.baseURL.JoinPath("/lock")
	_, err := sc.client.Post(url.String(), "application/json", nil)
	if err != nil {
		return err
	}
	return nil
}

func (sc *ServerClient) unlock(ctx context.Context) error {
	url := sc.baseURL.JoinPath("/unlock")
	_, err := sc.client.Post(url.String(), "application/json", nil)
	if err != nil {
		return err
	}
	return nil
}

func (sc *ServerClient) download(ctx context.Context) (*ExportedDatabase, error) {
	url := sc.baseURL.JoinPath("/database")
	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := sc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var exported ExportedDatabase
	err = json.NewDecoder(resp.Body).Decode(&exported)
	if err != nil {
		return nil, err
	}
	return &exported, nil
}

func (sc *ServerClient) upload(ctx context.Context, ed ExportedDatabase) error {
	url := sc.baseURL.JoinPath("/database")
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(ed)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", url.String(), buf)
	if err != nil {
		return err
	}
	resp, err := sc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	return nil
}
