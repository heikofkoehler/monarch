// Package client provides an HTTP client for the Monarch Money API.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	baseURL      = "https://api.monarch.com"
	loginURL     = baseURL + "/auth/login/"
	graphqlURL   = baseURL + "/graphql"
	sessionFile  = ".mm/session.json"
	userAgent    = "MonarchMoneyAPI (https://github.com/hammem/monarchmoney)"
)

// Client holds auth state and HTTP configuration for the Monarch Money API.
type Client struct {
	token      string
	httpClient *http.Client
}

// New creates a new Client with a default 30-second timeout.
func New() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetToken sets the auth token directly (e.g. loaded from a session file).
func (c *Client) SetToken(token string) {
	c.token = token
}

// Token returns the current auth token.
func (c *Client) Token() string {
	return c.token
}

type loginRequest struct {
	Password    string `json:"password"`
	SupportsMFA bool   `json:"supports_mfa"`
	TrustedDevice bool `json:"trusted_device"`
	Username    string `json:"username"`
	TOTP        string `json:"totp,omitempty"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type sessionData struct {
	Token string `json:"token"`
}

// Login authenticates with Monarch Money.
// If the server responds with 403, it returns ErrMFARequired.
func (c *Client) Login(email, password, totp string) error {
	req := loginRequest{
		Password:      password,
		SupportsMFA:   true,
		TrustedDevice: false,
		Username:      email,
	}
	if totp != "" {
		req.TOTP = totp
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, loginURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return ErrMFARequired
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}
	if lr.Token == "" {
		return fmt.Errorf("no token in login response")
	}
	c.token = lr.Token
	return nil
}

// ErrMFARequired is returned by Login when MFA is required.
var ErrMFARequired = fmt.Errorf("multi-factor authentication required")

// SaveSession writes the auth token to disk.
func (c *Client) SaveSession() error {
	if err := os.MkdirAll(".mm", 0700); err != nil {
		return err
	}
	data, err := json.Marshal(sessionData{Token: c.token})
	if err != nil {
		return err
	}
	return os.WriteFile(sessionFile, data, 0600)
}

// LoadSession reads a previously saved auth token from disk.
// Returns false if no session file exists.
func (c *Client) LoadSession() (bool, error) {
	raw, err := os.ReadFile(sessionFile)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var sd sessionData
	if err := json.Unmarshal(raw, &sd); err != nil {
		return false, err
	}
	if sd.Token == "" {
		return false, nil
	}
	c.token = sd.Token
	return true, nil
}

// DeleteSession removes the session file.
func (c *Client) DeleteSession() error {
	err := os.Remove(sessionFile)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// graphqlRequest is the payload sent to the GraphQL endpoint.
type graphqlRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

// GraphQLCall sends a GraphQL query to Monarch Money and returns the parsed "data" object.
func (c *Client) GraphQLCall(operationName, query string, variables map[string]any) (map[string]json.RawMessage, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated: call Login() first or load a session")
	}

	payload, err := json.Marshal(graphqlRequest{
		Query:         query,
		OperationName: operationName,
		Variables:     variables,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, graphqlURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graphql request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("graphql HTTP %d: %s\n%s", resp.StatusCode, resp.Status, body)
	}

	var envelope struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode graphql response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", envelope.Errors[0].Message)
	}
	return envelope.Data, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Client-Platform", "web")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Token "+c.token)
	}
}
