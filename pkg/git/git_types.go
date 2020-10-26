package git

type Repo struct {
	// Name describes name of the git repository
	Name string `json:"name,omitempty"`
	// URL is a full link to the git repository
	URL string `json:"url,omitempty"`

	BaseRef string `json:"base_ref,omitempty"`
}
