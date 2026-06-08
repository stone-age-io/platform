// Package pbclient is a tiny PocketBase REST client used by leaf-sync. PocketBase
// has no official Go client SDK (official SDKs are JS/Dart), and leaf-sync only
// needs a handful of read endpoints plus auth-with-password, so this stays
// deliberately small.
package pbclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Record is a generic PocketBase record. leaf-sync mirrors arbitrary collections,
// so records are kept as untyped maps.
type Record = map[string]any

type Client struct {
	baseURL    string
	collection string
	identity   string
	password   string
	token      string
	http       *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// AuthWithPassword authenticates against an auth collection, stores the token for
// subsequent requests, and returns the authenticated record. Credentials are
// retained so the client can transparently re-authenticate on token expiry.
func (c *Client) AuthWithPassword(ctx context.Context, collection, identity, password string) (Record, error) {
	c.collection = collection
	c.identity = identity
	c.password = password
	return c.authenticate(ctx)
}

func (c *Client) authenticate(ctx context.Context) (Record, error) {
	body, _ := json.Marshal(map[string]string{"identity": c.identity, "password": c.password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/collections/"+url.PathEscape(c.collection)+"/auth-with-password",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("auth failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var out struct {
		Token  string `json:"token"`
		Record Record `json:"record"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	c.token = out.Token
	return out.Record, nil
}

// get performs an authenticated GET, transparently re-authenticating once on 401
// (handles token expiry for the long-running `run` daemon).
func (c *Client) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	full := c.baseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	do := func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
		if err != nil {
			return nil, err
		}
		if c.token != "" {
			req.Header.Set("Authorization", c.token)
		}
		return c.http.Do(req)
	}

	resp, err := do()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized && c.identity != "" {
		resp.Body.Close()
		if _, err := c.authenticate(ctx); err != nil {
			return nil, err
		}
		resp, err = do()
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s (%d): %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}

// GetOne fetches a single record by id.
func (c *Client) GetOne(ctx context.Context, collection, id string) (Record, error) {
	b, err := c.get(ctx, "/api/collections/"+url.PathEscape(collection)+"/records/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	var rec Record
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// ListResult mirrors PocketBase's paginated list response.
type ListResult struct {
	Page       int      `json:"page"`
	PerPage    int      `json:"perPage"`
	TotalItems int      `json:"totalItems"`
	TotalPages int      `json:"totalPages"`
	Items      []Record `json:"items"`
}

// List fetches one page of records from a collection.
func (c *Client) List(ctx context.Context, collection string, page, perPage int, filter string) (*ListResult, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("perPage", strconv.Itoa(perPage))
	if filter != "" {
		q.Set("filter", filter)
	}
	b, err := c.get(ctx, "/api/collections/"+url.PathEscape(collection)+"/records", q)
	if err != nil {
		return nil, err
	}
	var res ListResult
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// GetRaw fetches an arbitrary authenticated endpoint and returns the raw body
// (used for the custom /api/leaf/operator-jwt route).
func (c *Client) GetRaw(ctx context.Context, path string) ([]byte, error) {
	return c.get(ctx, path, nil)
}
