package explainer

type Explanation struct {
	Name        string        `json:"name"`
	FullName    string        `json:"fullName"`
	Type        string        `json:"type"`
	Description string        `json:"description"`
	Fields      []Explanation `json:"fields"`
}

type Explainer interface {
	Explain(resourceNames []string) []Explanation
}
