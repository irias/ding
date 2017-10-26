package sherpa

// Documentation object, to be returned by a Sherpa API "_docs" function.
type Doc struct {
	Title     string         `json:"title"`
	Text      string         `json:"text"`
	Functions []*FunctionDoc `json:"functions"`
	Sections  []*Doc         `json:"sections"`
}

// Documentation for a single function Name.
// Text should be in markdown. The first line should be a synopsis showing parameters including types, and the return types.
type FunctionDoc struct {
	Name string `json:"name"`
	Text string `json:"text"`
}
