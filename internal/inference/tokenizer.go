package inference

// Tokenizer handles tokenization via llama-server
type Tokenizer struct {
	serverURL string
}

// NewTokenizer creates a new tokenizer instance
func NewTokenizer(serverURL string) *Tokenizer {
	return &Tokenizer{
		serverURL: serverURL,
	}
}

// CountTokens counts tokens in a text string
func (t *Tokenizer) CountTokens(text string) (int, error) {
	// Simple estimation: ~4 chars per token
	return len(text) / 4, nil
}
