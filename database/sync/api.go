package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	resp, err := sc.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload changes (status code %d): %s", resp.StatusCode, string(body))
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
	resp, err := sc.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload changes (status code %d): %s", resp.StatusCode, string(body))
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
	resp, err := sc.client.Do(req)
	if err != nil {
		return ExportedDatabase{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ExportedDatabase{}, fmt.Errorf("failed to download database (status code %d): %s", resp.StatusCode, string(body))
	}
	var exported ExportedDatabase
	err = json.NewDecoder(resp.Body).Decode(&exported)
	if err != nil {
		return ExportedDatabase{}, err
	}
	return exported, nil
}

func (sc *ServerClient) uploadChanges(ctx context.Context, changes Changes) error {
	url := sc.baseURL.JoinPath("/database/changes")
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(changes)
	if err != nil {
		return err
	}
	log.Printf("url = %v", url)
	req, err := http.NewRequestWithContext(ctx, "POST", url.String(), buf)
	if err != nil {
		panic(err)
	}
	if sc.token.Empty() {
		panic("token is empty")
	}
	req.Header.Set("X-API-Token", sc.token.String())
	req.Header.Set("Content-Type", "application/json")
	resp, err := sc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload changes (status code %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (sc *ServerClient) SyncDatabase(ctx context.Context, originalED ExportedDatabase, db *database.Database, resolver func(MergeConflicts) (Changes, error), status chan<- string) (Changes, ExportedDatabase, error) {
	if status != nil {
		status <- "施錠"
	}
	err := sc.lock(ctx)
	if err != nil {
		return Changes{}, ExportedDatabase{}, fmt.Errorf("lock: %w", err)
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
		return Changes{}, ExportedDatabase{}, fmt.Errorf("download: %w", err1)
	}
	if err2 != nil {
		return Changes{}, ExportedDatabase{}, fmt.Errorf("local export: %w", err2)
	}
	if status != nil {
		status <- "マージ"
	}

	if len(originalED.Sessions) == 0 && len(originalED.Timeframes) == 0 {
		originalED = serverED
	}
	log.Printf("serverED has %d sessions and %d timeframes", len(serverED.Sessions), len(serverED.Timeframes))
	changes, conflicts := Merge(originalED, localED, serverED)
	log.Printf("num of conflicts: %d", len(conflicts.Sessions)+len(conflicts.Timeframes))
	if len(conflicts.Sessions) > 0 || len(conflicts.Timeframes) > 0 {
		if resolver == nil {
			return Changes{}, ExportedDatabase{}, fmt.Errorf("conflicts detected, but no resolver provided")
		} else {
			changes2, err := resolver(conflicts)
			if err != nil {
				return Changes{}, ExportedDatabase{}, fmt.Errorf("resolve conflicts: %w", err)
			}
			changes.Sessions = append(changes.Sessions, changes2.Sessions...)
			changes.Timeframes = append(changes.Timeframes, changes2.Timeframes...)
		}
	}

	for i, s := range changes.Sessions {
		log.Printf("change %d: session: %s", i, s)
	}
	for i, t := range changes.Timeframes {
		log.Printf("change %d: timeframe: %s", i, t)
	}

	if status != nil {
		status <- "更新"
	}
	var wg2 sync.WaitGroup
	wg2.Add(2)
	go func() {
		defer wg2.Done()
		err1 = sc.uploadChanges(ctx, changes)
	}()
	go func() {
		defer wg2.Done()
		// replace local database with server database
		err2 = ReplaceAndImport(db, serverED, changes)
	}()
	wg2.Wait()
	if err1 != nil {
		return Changes{}, ExportedDatabase{}, fmt.Errorf("upload changes: %w", err1)
	}
	if err2 != nil {
		return Changes{}, ExportedDatabase{}, fmt.Errorf("local replace and import: %w", err2)
	}

	newED, err := Export(db)
	if err != nil {
		panic(err)
	}

	return changes, newED, nil
}
