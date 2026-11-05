// Package github provides the small GitHub Contents API surface used by the
// monitor writeback pipeline.
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultAPIURL    = "https://api.github.com"
	defaultOwner     = "AobaIwaki123"
	defaultRepo      = "taite-board"
	defaultBranch    = "main"
	defaultUserAgent = "taite-board-monitor"
	maxResponseSize  = 8 << 20
)

// ErrNotFound indicates that the requested repository file does not exist.
var ErrNotFound = errors.New("github file not found")

// APIError is returned when GitHub responds with a non-success status.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("github api returned status %d", e.StatusCode)
	}
	return fmt.Sprintf("github api returned status %d: %s", e.StatusCode, e.Message)
}

// IsConflict reports whether err is a GitHub optimistic-lock conflict.
func IsConflict(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict
}

// IsNotFound reports whether err means that a repository file was not found.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || func() bool {
		var apiErr *APIError
		return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
	}()
}

// CommitResult describes the commit returned by the Contents API.
type CommitResult struct {
	Path string `json:"path"`
	SHA  string `json:"sha"`
	URL  string `json:"url,omitempty"`
}

// FileMutation transforms the latest repository content into the content to
// commit. It is called again after a 409 conflict so a concurrent update is
// never silently overwritten with stale input.
type FileMutation func(current []byte) ([]byte, error)

// Client is a GitHub REST Contents API client.
type Client struct {
	token     string
	owner     string
	repo      string
	apiURL    string
	userAgent string
	branch    string
	http      *http.Client
}

// Config configures a GitHub Contents API client.
type Config struct {
	Token     string
	Owner     string
	Repo      string
	APIURL    string
	Branch    string
	UserAgent string
	HTTP      *http.Client
}

// NewClient constructs a client with explicit configuration.
func NewClient(cfg Config) *Client {
	owner := strings.TrimSpace(cfg.Owner)
	if owner == "" {
		owner = defaultOwner
	}
	repo := strings.TrimSpace(cfg.Repo)
	if repo == "" {
		repo = defaultRepo
	}
	apiURL := strings.TrimRight(strings.TrimSpace(cfg.APIURL), "/")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	branch := strings.TrimSpace(cfg.Branch)
	if branch == "" {
		branch = defaultBranch
	}
	userAgent := strings.TrimSpace(cfg.UserAgent)
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	httpClient := cfg.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	return &Client{
		token:     strings.TrimSpace(cfg.Token),
		owner:     owner,
		repo:      repo,
		apiURL:    apiURL,
		userAgent: userAgent,
		branch:    branch,
		http:      httpClient,
	}
}

// NewClientFromEnv returns nil when GITHUB_TOKEN is not configured. This
// keeps local development and tests explicit while enabling production
// writeback only when the Kubernetes Secret is mounted.
func NewClientFromEnv() *Client {
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		return nil
	}
	return NewClient(Config{
		Token:  token,
		Owner:  os.Getenv("GITHUB_OWNER"),
		Repo:   os.Getenv("GITHUB_REPOSITORY_NAME"),
		APIURL: os.Getenv("GITHUB_API_URL"),
		Branch: os.Getenv("GITHUB_BRANCH"),
	})
}

// UpdateFile reads the latest file, applies mutate, and commits the result.
// A single retry is performed after a 409 response using a freshly fetched
// file, which preserves optimistic-lock semantics under concurrent intake.
func (c *Client) UpdateFile(ctx context.Context, branch, filePath, message string, mutate FileMutation) (CommitResult, error) {
	if c == nil {
		return CommitResult{}, errors.New("github client is nil")
	}
	if mutate == nil {
		return CommitResult{}, errors.New("file mutation is required")
	}
	if strings.TrimSpace(branch) == "" {
		branch = c.branch
	}
	if strings.TrimSpace(branch) == "" {
		branch = defaultBranch
	}
	if strings.TrimSpace(filePath) == "" {
		return CommitResult{}, errors.New("file path is required")
	}
	if strings.TrimSpace(message) == "" {
		return CommitResult{}, errors.New("commit message is required")
	}

	for attempt := 0; attempt < 2; attempt++ {
		file, err := c.getFile(ctx, branch, filePath)
		if err != nil && !IsNotFound(err) {
			return CommitResult{}, err
		}
		if IsNotFound(err) {
			file = repositoryFile{Path: filePath}
		}

		updated, err := mutate(file.Content)
		if err != nil {
			return CommitResult{}, fmt.Errorf("mutating %s: %w", filePath, err)
		}

		result, err := c.putFile(ctx, branch, filePath, message, updated, file.SHA)
		if err == nil {
			return result, nil
		}
		if !IsConflict(err) || attempt == 1 {
			return CommitResult{}, err
		}
	}

	return CommitResult{}, errors.New("github file update exhausted retries")
}

type repositoryFile struct {
	Path    string
	SHA     string
	Content []byte
}

type contentsResponse struct {
	Path     string `json:"path"`
	SHA      string `json:"sha"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	HTMLURL  string `json:"html_url"`
}

func (c *Client) getFile(ctx context.Context, branch, filePath string) (repositoryFile, error) {
	endpoint, err := c.contentsURL(filePath, branch)
	if err != nil {
		return repositoryFile{}, err
	}
	request, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return repositoryFile{}, err
	}

	response, err := c.http.Do(request)
	if err != nil {
		return repositoryFile{}, fmt.Errorf("getting %s: %w", filePath, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return repositoryFile{}, c.apiError(response)
	}

	var payload contentsResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseSize)).Decode(&payload); err != nil {
		return repositoryFile{}, fmt.Errorf("decoding GitHub file response: %w", err)
	}
	if payload.Encoding != "" && payload.Encoding != "base64" {
		return repositoryFile{}, fmt.Errorf("unsupported GitHub content encoding %q", payload.Encoding)
	}
	content, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(payload.Content), ""))
	if err != nil {
		return repositoryFile{}, fmt.Errorf("decoding GitHub file content: %w", err)
	}

	return repositoryFile{
		Path:    payload.Path,
		SHA:     payload.SHA,
		Content: content,
	}, nil
}

type putRequest struct {
	Message string `json:"message"`
	Content string `json:"content"`
	SHA     string `json:"sha,omitempty"`
	Branch  string `json:"branch"`
}

type putResponse struct {
	Content struct {
		Path string `json:"path"`
		SHA  string `json:"sha"`
	} `json:"content"`
	Commit struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
	} `json:"commit"`
}

func (c *Client) putFile(ctx context.Context, branch, filePath, message string, content []byte, sha string) (CommitResult, error) {
	endpoint, err := c.contentsURL(filePath, branch)
	if err != nil {
		return CommitResult{}, err
	}
	body, err := json.Marshal(putRequest{
		Message: message,
		Content: base64.StdEncoding.EncodeToString(content),
		SHA:     sha,
		Branch:  branch,
	})
	if err != nil {
		return CommitResult{}, fmt.Errorf("encoding GitHub update request: %w", err)
	}
	request, err := c.newRequest(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return CommitResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return CommitResult{}, fmt.Errorf("updating %s: %w", filePath, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return CommitResult{}, c.apiError(response)
	}

	var payload putResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseSize)).Decode(&payload); err != nil {
		return CommitResult{}, fmt.Errorf("decoding GitHub update response: %w", err)
	}
	commitSHA := payload.Commit.SHA
	if commitSHA == "" {
		commitSHA = payload.Content.SHA
	}
	return CommitResult{
		Path: payload.Content.Path,
		SHA:  commitSHA,
		URL:  payload.Commit.HTMLURL,
	}, nil
}

func (c *Client) contentsURL(filePath, branch string) (string, error) {
	cleanPath := strings.Trim(filePath, "/")
	if cleanPath == "" || strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("invalid repository file path %q", filePath)
	}
	segments := strings.Split(cleanPath, "/")
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			return "", fmt.Errorf("invalid repository file path %q", filePath)
		}
		escaped = append(escaped, url.PathEscape(segment))
	}
	return fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		c.apiURL,
		url.PathEscape(c.owner),
		url.PathEscape(c.repo),
		strings.Join(escaped, "/"),
		url.QueryEscape(branch),
	), nil
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("creating GitHub request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Authorization", "Bearer "+c.token)
	return request, nil
}

func (c *Client) apiError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	var payload struct {
		Message string `json:"message"`
	}
	message := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &payload) == nil && payload.Message != "" {
		message = payload.Message
	}
	if response.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ErrNotFound, message)
	}
	return &APIError{StatusCode: response.StatusCode, Message: message}
}
