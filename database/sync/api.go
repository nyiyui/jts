package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"

	"nyiyui.ca/jts/database"
	"nyiyui.ca/jts/tokens"
)

type ServerClient struct {
	client  *http.Client
	baseURL *url.URL
	token   tokens.Token
}

func NewServerClient(client *http.Client, baseURL *url.URL, token tokens.Token) *ServerClient {
	return &ServerClient{
		client:  client,
		baseURL: baseURL,
		token:   token,
	}
}

func (sc *ServerClient) lock(ctx context.Context) error {
	url := sc.baseURL.JoinPath("/lock")
	req, err := http.NewRequestWithContext(ctx, "POST", url.String(), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("X-API-Token", sc.token.String())
	_, err = sc.client.Do(req)
	if err != nil {
		return err
	}
	return nil
}

func (sc *ServerClient) unlock(ctx context.Context) error {
	url := sc.baseURL.JoinPath("/unlock")
	req, err := http.NewRequestWithContext(ctx, "POST", url.String(), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("X-API-Token", sc.token.String())
	_, err = sc.client.Do(req)
	if err != nil {
		return err
	}
	return nil
}

func (sc *ServerClient) download(ctx context.Context) (ExportedDatabase, error) {
	url := sc.baseURL.JoinPath("/database")
	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("X-API-Token", sc.token.String())
	_, err = sc.client.Do(req)
	if err != nil {
		return ExportedDatabase{}, err
	}
	resp, err := sc.client.Do(req)
	if err != nil {
		return ExportedDatabase{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ExportedDatabase{}, nil
	}
	var exported ExportedDatabase
	err = json.NewDecoder(resp.Body).Decode(&exported)
	if err != nil {
		return ExportedDatabase{}, err
	}
	return exported, nil
}

func (sc *ServerClient) upload(ctx context.Context, ed ExportedDatabase) error {
	url := sc.baseURL.JoinPath("/database")
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(ed)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", url.String(), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("X-API-Token", sc.token.String())
	_, err = sc.client.Do(req)
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

func (sc *ServerClient) uploadChanges(ctx context.Context, changes Changes) error {
	url := sc.baseURL.JoinPath("/database/changes")
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(changes)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url.String(), buf)
	if err != nil {
		panic(err)
	}
	req.Header.Set("X-API-Token", sc.token.String())
	_, err = sc.client.Do(req)
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

func (sc *ServerClient) SyncDatabase(ctx context.Context, originalED ExportedDatabase, db *database.Database, status chan<- string) (Changes, error) {
	if status != nil {
		status <- "施錠"
	}
	err := sc.lock(ctx)
	if err != nil {
		return Changes{}, err
	}
	defer func() {
		if status != nil {
			status <- "解錠"
		}
		err := sc.unlock(ctx)
		if err != nil {
			log.Printf("SyncDatabase: unlock: %s", err)
		}
	}()

	if status != nil {
		status <- "取得"
	}
	var serverED, localED ExportedDatabase
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		serverED, err1 = sc.download(ctx)
		log.Printf("downloaded: %#v", serverED)
	}()
	go func() {
		defer wg.Done()
		localED, err2 = Export(db)
	}()
	wg.Wait()
	if err1 != nil {
		return Changes{}, fmt.Errorf("download: %w", err1)
	}
	if err2 != nil {
		return Changes{}, fmt.Errorf("local export: %w", err2)
	}
	if status != nil {
		status <- "マージ"
	}

	if len(originalED.Sessions) == 0 && len(originalED.Timeframes) == 0 {
		originalED = serverED
	}
	changes, conflicts := Merge(originalED, localED, serverED)
	if len(conflicts.Sessions) > 0 || len(conflicts.Timeframes) > 0 {
		return Changes{}, fmt.Errorf("conflicts: %v", conflicts)
	}

	if status != nil {
		status <- "更新"
	}
	log.Printf("changes: %#v", changes)
	err = sc.uploadChanges(ctx, changes)
	if err != nil {
		return Changes{}, fmt.Errorf("upload changes: %w", err)
	}
	var wg2 sync.WaitGroup
	wg2.Add(2)
	go func() {
		defer wg2.Done()
	}()
	go func() {
		defer wg2.Done()
		// replace local database with server database
		// TODO
		//err = ReplaceAndImport(db, serverED, changes)
		//if err != nil {
		//	return Changes{}, fmt.Errorf("replace: %w", err)
		//}
	}()

	return changes, nil
}
