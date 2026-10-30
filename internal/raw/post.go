package raw

// Post represents an immutable raw post snapshot fetched from an external source (ADR-0002).
type Post struct {
	SchemaVersion int      `json:"schema_version"`
	Source        string   `json:"source"`
	SourceID      string   `json:"source_id"`
	URL           string   `json:"url"`
	Author        string   `json:"author"`
	AuthorURL     string   `json:"author_url,omitempty"`
	Text          string   `json:"text"`
	Images        []string `json:"images"`
	FetchedAt     string   `json:"fetched_at"`
}

// RunManifest records metadata and results of a batch fetch run (ADR-0002).
type RunManifest struct {
	SchemaVersion int               `json:"schema_version"`
	RunID         string            `json:"run_id"`
	Source        string            `json:"source"`
	FetchedAt     string            `json:"fetched_at"`
	InputCount    int               `json:"input_count"`
	SuccessCount  int               `json:"success_count"`
	FailedCount   int               `json:"failed_count"`
	Failures      map[string]string `json:"failures,omitempty"`
	SavedPosts    []string          `json:"saved_posts,omitempty"`
}
