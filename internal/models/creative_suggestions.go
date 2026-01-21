package models

type CreativeSuggestionsResponse struct {
	Data CreativeSuggestionsData `json:"data"`
}

type CreativeSuggestionsData struct {
	Results []CreativeFileSuggestionResult `json:"results"`
}

type CreativeFileSuggestionResult struct {
	FileName      string           `json:"file_name"`
	Status        string           `json:"status"`
	Error         string           `json:"error,omitempty"`
	ExtractedText string           `json:"extracted_text,omitempty"`
	Keywords      []string         `json:"keywords,omitempty"`
	Suggestions   []VenueSuggestion `json:"suggestions,omitempty"`
}

type VenueSuggestion struct {
	VenueID             int      `json:"venue_id"`
	Name                string   `json:"name"`
	MatchedSubCategories []string `json:"matched_sub_categories,omitempty"`
	Score               float64  `json:"score"`
	Reasons             []string `json:"reasons,omitempty"`
}
