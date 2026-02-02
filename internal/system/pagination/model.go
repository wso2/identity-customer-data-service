package pagination

type Pagination struct {
	Count          int    `json:"count"`
	PageSize       int    `json:"page_size"`
	NextCursor     string `json:"next_cursor,omitempty"`
	PreviousCursor string `json:"previous_cursor,omitempty"`
}
