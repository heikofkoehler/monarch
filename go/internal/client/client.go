// Package client provides an HTTP client for the Monarch Money API.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	baseURL     = "https://api.monarch.com"
	loginURL    = baseURL + "/auth/login/"
	graphqlURL  = baseURL + "/graphql"
	sessionFile = ".mm/session.json"
	userAgent   = "MonarchMoneyAPI (https://github.com/hammem/monarchmoney)"
)

// consoleSnippet extracts the Monarch session token and copies it to the clipboard.
const consoleSnippet = `(function(){
  let token = "";
  try {
    const root = JSON.parse(localStorage.getItem("persist:root") || "{}");
    token = JSON.parse(root.user || "{}").token || "";
  } catch(e) {}
  if (!token) {
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      const v = localStorage.getItem(k);
      if (k && k.toLowerCase().includes("token") && v && v.length > 20 && !v.startsWith("{")) {
        token = v; break;
      }
    }
  }
  if (!token) { console.error("Token not found — are you logged in?"); return; }
  navigator.clipboard.writeText(token).then(
    () => console.log("Token copied to clipboard!"),
    () => { console.log("Copy failed — token is:"); console.log(token); }
  );
})()`

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
	Password      string `json:"password"`
	SupportsMFA   bool   `json:"supports_mfa"`
	TrustedDevice bool   `json:"trusted_device"`
	Username      string `json:"username"`
	TOTP          string `json:"totp,omitempty"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type sessionData struct {
	Token string `json:"token"`
}

// Login authenticates with Monarch Money using email and password.
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
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
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

// LoginWithGoogle opens app.monarch.com in Chrome, prints a JavaScript snippet
// the user runs in the browser console to copy their Monarch token to the clipboard,
// then reads the token automatically from the clipboard via pbpaste.
func (c *Client) LoginWithGoogle(ctx context.Context) error {
	fmt.Println("Opening app.monarch.com in Chrome...")
	fmt.Println()
	fmt.Println("Once the page loads:")
	fmt.Println("  1. Open the browser console  (Cmd+Option+J)")
	fmt.Println("  2. Paste the snippet below and press Enter")
	fmt.Println("     → It will copy your Monarch token to the clipboard")
	fmt.Println()
	fmt.Println(consoleSnippet)
	fmt.Println()

	_ = openBrowser("https://app.monarch.com")

	prompt("Press Enter after the console says \"Token copied to clipboard!\"...")

	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		// pbpaste not available (non-macOS) — fall back to manual paste.
		token := prompt("Paste token here: ")
		if token == "" {
			return fmt.Errorf("no token provided")
		}
		c.token = token
		return nil
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return fmt.Errorf("clipboard is empty — did the snippet run successfully?")
	}
	c.token = token
	return nil
}

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
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("graphql HTTP %d: %s\n%s", resp.StatusCode, resp.Status, b)
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

func prompt(label string) string {
	fmt.Fprint(os.Stdout, label)
	sc := bufio.NewScanner(os.Stdin)
	sc.Scan()
	return strings.TrimSpace(sc.Text())
}

// openBrowser opens the given URL, preferring Chrome on macOS.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		// Prefer Chrome; fall back to system default if not installed.
		if err := exec.Command("open", "-a", "Google Chrome", url).Start(); err == nil {
			return nil
		}
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
