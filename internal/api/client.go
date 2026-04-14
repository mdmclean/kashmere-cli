// internal/api/client.go
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mdmclean/kashmere-cli/internal/crypto"
)

// Client is an authenticated Kashmere API client with E2E encryption.
type Client struct {
	baseURL      string
	apiKey       string
	bearerToken  string
	encKey       []byte // 32-byte AES key derived from passphrase+salt; nil = no encryption
	http         *http.Client
}

// New creates an API client. encKey may be nil if encryption is not needed
// (e.g., for setup before the key is derived).
func New(baseURL, apiKey string, encKey []byte) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		encKey:  encKey,
		http:    &http.Client{},
	}
}

// SetBearerToken sets a JWT bearer token for authentication.
func (c *Client) SetBearerToken(token string) {
	c.bearerToken = token
}

// encryptedPaths lists path prefixes whose request/response bodies are E2E encrypted.
var encryptedPaths = []string{
	"/portfolios", "/goals", "/mortgages", "/cashflows", "/settings",
}

func (c *Client) shouldEncrypt(path string) bool {
	if c.encKey == nil {
		return false
	}
	for _, p := range encryptedPaths {
		if len(path) >= len(p) && path[:len(p)] == p {
			return true
		}
	}
	return false
}

// Get makes an authenticated GET request and decodes the JSON response into result.
func (c *Client) Get(path string, result any) error {
	resp, err := c.do("GET", path, nil)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return parseAPIError(resp.StatusCode, body)
	}
	raw := json.RawMessage(body)
	if c.shouldEncrypt(path) {
		return c.decryptResponse(raw, result)
	}
	return json.Unmarshal(body, result)
}

// Post makes an authenticated POST, encrypting the body if needed.
func (c *Client) Post(path string, body any, result any) error {
	return c.write("POST", path, body, result)
}

// Put makes an authenticated PUT, encrypting the body if needed.
func (c *Client) Put(path string, body any, result any) error {
	return c.write("PUT", path, body, result)
}

// GetSnapshots fetches and decrypts portfolio snapshots.
// Unlike other encrypted resources, snapshot encryption only covers financial
// fields (totalValue); metadata (portfolioId, timestamp) stays plaintext so
// the server can filter without decrypting.
func (c *Client) GetSnapshots(path string, result any) error {
	resp, err := c.do("GET", path, nil)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return parseAPIError(resp.StatusCode, body)
	}
	if c.encKey == nil {
		return json.Unmarshal(body, result)
	}
	return c.decryptSnapshotResponse(json.RawMessage(body), result)
}

// PostSnapshot creates a snapshot with encryption that preserves portfolioId
// and timestamp as plaintext for server-side date filtering, while encrypting
// the financial value (totalValue).
func (c *Client) PostSnapshot(portfolioID, timestamp string, totalValue float64) error {
	dateStr := timestamp
	if len(dateStr) >= 10 {
		dateStr = dateStr[:10]
	}
	id := "snapshot-" + portfolioID + "-" + dateStr

	var body map[string]any
	if c.encKey != nil {
		sensitive := map[string]any{"totalValue": totalValue}
		sensitiveJSON, err := json.Marshal(sensitive)
		if err != nil {
			return err
		}
		ciphertext, err := crypto.Encrypt(c.encKey, string(sensitiveJSON))
		if err != nil {
			return err
		}
		body = map[string]any{
			"id":          id,
			"portfolioId": portfolioID,
			"timestamp":   timestamp,
			"_encrypted":  ciphertext,
		}
	} else {
		body = map[string]any{
			"id":          id,
			"portfolioId": portfolioID,
			"timestamp":   timestamp,
			"totalValue":  totalValue,
		}
	}

	resp, err := c.do("POST", "/history/snapshots", body)
	if err != nil {
		return err
	}
	io.ReadAll(resp.Body) //nolint:errcheck
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("snapshot creation failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// decryptSnapshotResponse decrypts an array or single snapshot response.
func (c *Client) decryptSnapshotResponse(raw json.RawMessage, result any) error {
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		resultSlice := make([]json.RawMessage, len(arr))
		for i, item := range arr {
			dec, err := c.decryptSnapshotDoc(item)
			if err != nil {
				return err
			}
			resultSlice[i] = json.RawMessage(dec)
		}
		combined, err := json.Marshal(resultSlice)
		if err != nil {
			return err
		}
		return json.Unmarshal(combined, result)
	}
	dec, err := c.decryptSnapshotDoc(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(dec), result)
}

// decryptSnapshotDoc decrypts a single snapshot document, merging plaintext
// metadata (id, accountId, portfolioId, timestamp) with the decrypted financial
// fields (totalValue). Falls back gracefully for unencrypted legacy snapshots.
func (c *Client) decryptSnapshotDoc(raw json.RawMessage) (string, error) {
	var wrapper struct {
		Encrypted string `json:"_encrypted"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil || wrapper.Encrypted == "" {
		return string(raw), nil // unencrypted legacy snapshot — return as-is
	}
	plaintext, err := crypto.Decrypt(c.encKey, wrapper.Encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt snapshot: %w", err)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(plaintext), &obj); err != nil {
		return "", err
	}
	// Merge plaintext metadata from the outer document
	var outer map[string]any
	if err := json.Unmarshal(raw, &outer); err == nil {
		for k, v := range outer {
			if k != "_encrypted" {
				obj[k] = v
			}
		}
	}
	out, err := json.Marshal(obj)
	return string(out), err
}

// Delete makes an authenticated DELETE request.
func (c *Client) Delete(path string) error {
	resp, err := c.do("DELETE", path, nil)
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return parseAPIError(resp.StatusCode, body)
	}
	return nil
}

func (c *Client) write(method, path string, body any, result any) error {
	var payload any = body
	if c.shouldEncrypt(path) {
		enc, err := c.encryptBody(body)
		if err != nil {
			return fmt.Errorf("encrypting request: %w", err)
		}
		payload = enc
	}
	resp, err := c.do(method, path, payload)
	if err != nil {
		return err
	}
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return parseAPIError(resp.StatusCode, respBody)
	}
	if result == nil || resp.StatusCode == 204 {
		return nil
	}
	raw := json.RawMessage(respBody)
	if c.shouldEncrypt(path) {
		return c.decryptResponse(raw, result)
	}
	return json.Unmarshal(respBody, result)
}

func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	return c.http.Do(req)
}

// encryptBody wraps all fields except id/accountId in _encrypted.
func (c *Client) encryptBody(v any) (encryptedDoc, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return encryptedDoc{}, err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return encryptedDoc{}, err
	}

	id, _ := obj["id"].(string)
	accountID, _ := obj["accountId"].(string)

	toEncrypt := make(map[string]any)
	for k, v := range obj {
		if k != "id" && k != "accountId" {
			toEncrypt[k] = v
		}
	}
	plain, err := json.Marshal(toEncrypt)
	if err != nil {
		return encryptedDoc{}, err
	}
	ciphertext, err := crypto.Encrypt(c.encKey, string(plain))
	if err != nil {
		return encryptedDoc{}, err
	}
	return encryptedDoc{ID: id, AccountID: accountID, Encrypted: ciphertext}, nil
}

// decryptResponse decrypts a single object or array of objects.
func (c *Client) decryptResponse(raw json.RawMessage, result any) error {
	// Try array first
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		resultSlice := make([]json.RawMessage, len(arr))
		for i, item := range arr {
			var doc encryptedDoc
			if json.Unmarshal(item, &doc) == nil && doc.Encrypted != "" {
				decrypted, err := c.decryptDoc(doc)
				if err != nil {
					return err
				}
				resultSlice[i] = json.RawMessage(decrypted)
			} else {
				resultSlice[i] = item
			}
		}
		// Marshal back and unmarshal into result
		combined, err := json.Marshal(resultSlice)
		if err != nil {
			return err
		}
		return json.Unmarshal(combined, result)
	}

	// Single object
	var doc encryptedDoc
	if json.Unmarshal(raw, &doc) == nil && doc.Encrypted != "" {
		decrypted, err := c.decryptDoc(doc)
		if err != nil {
			return err
		}
		return json.Unmarshal([]byte(decrypted), result)
	}
	return json.Unmarshal(raw, result)
}

// decryptDoc decrypts a single encryptedDoc and merges id/accountId back.
func (c *Client) decryptDoc(doc encryptedDoc) (string, error) {
	plaintext, err := crypto.Decrypt(c.encKey, doc.Encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(plaintext), &obj); err != nil {
		return "", err
	}
	if doc.ID != "" {
		obj["id"] = doc.ID
	}
	if doc.AccountID != "" {
		obj["accountId"] = doc.AccountID
	}
	out, err := json.Marshal(obj)
	return string(out), err
}

// MergeAndUpdate fetches the current object at GET path/id, merges updates,
// then PUT path/id with the full merged object. Required for E2E encryption
// since the server cannot do partial updates on encrypted blobs.
func (c *Client) MergeAndUpdate(getPath string, updates map[string]any, result any) error {
	var existing map[string]any
	if err := c.Get(getPath, &existing); err != nil {
		return fmt.Errorf("fetching current record: %w", err)
	}
	for k, v := range updates {
		existing[k] = v
	}
	return c.Put(getPath, existing, result)
}

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.Status, e.Message)
}

func parseAPIError(status int, body []byte) error {
	var errResp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(body, &errResp)
	msg := errResp.Error
	if msg == "" {
		msg = string(body)
	}
	return &apiError{Status: status, Message: msg}
}
