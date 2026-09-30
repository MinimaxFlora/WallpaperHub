package server

// imageView is the JSON representation of a wallpaper.
type imageView struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Category        string   `json:"category"`
	Tags            []string `json:"tags"`
	Orientation     string   `json:"orientation"`
	Width           int      `json:"width"`
	Height          int      `json:"height"`
	Format          string   `json:"format"`
	Bytes           int64    `json:"bytes"`
	Hash            string   `json:"hash"`
	URL             string   `json:"url"`
	PageURL         string   `json:"page_url"`
	Mode            string   `json:"mode,omitempty"`
	ManifestVersion int      `json:"manifest_version,omitempty"`
}

// errorResponse is the envelope for every API error.
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// listResponse is the paginated payload of GET /v1/images.
type listResponse struct {
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Items    []imageView `json:"items"`
}

// countResponse is the payload of GET /v1/tags and GET /v1/categories.
type countResponse struct {
	Items []countItem `json:"items"`
}

type countItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// healthResponse is the payload of GET /healthz.
type healthResponse struct {
	Status              string `json:"status"`
	Images              int    `json:"images"`
	ManifestGeneratedAt string `json:"manifest_generated_at,omitempty"`
}

// indexResponse is the payload of GET /.
type indexResponse struct {
	Service   string   `json:"service"`
	Version   int      `json:"manifest_version"`
	Images    int      `json:"images"`
	Endpoints []string `json:"endpoints"`
}
