package inference

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"math"
	"unicode/utf8"
)

const maxONNXEmbeddingValues = 32768
const maxONNXEmbeddingPromptBytes = 768 << 10

// JSON escaping can expand an individually valid input sixfold. Build each
// bounded frame from whole encoded strings instead of estimating raw bytes or
// truncating input to fit a fixed-count batch.
func nextONNXEmbeddingBatch(texts []string, maximum int) (int, json.RawMessage, error) {
	prompt := []byte(`{"input":[`)
	count := 0
	for _, input := range texts[:min(len(texts), maximum)] {
		if input == "" || len(input) > 32768 || !utf8.ValidString(input) {
			return 0, nil, errors.New("ONNX embedding text exceeds input bounds")
		}
		encoded, err := json.Marshal(input)
		if err != nil {
			return 0, nil, err
		}
		separator := 0
		if count > 0 {
			separator = 1
		}
		if len(prompt)+separator+len(encoded)+2 > maxONNXEmbeddingPromptBytes {
			break
		}
		if separator != 0 {
			prompt = append(prompt, ',')
		}
		prompt = append(prompt, encoded...)
		count++
	}
	if count == 0 {
		return 0, nil, errors.New("ONNX embedding input does not fit the worker frame")
	}
	return count, append(prompt, ']', '}'), nil
}

func validateONNXEmbeddingPrompt(req ProviderRequest, descriptor ProviderDescriptor) (int, error) {
	if req.ContextLength != descriptor.ContextLength || len(req.Prompt) > maxONNXEmbeddingPromptBytes || jsonwire.Validate(req.Prompt) != nil {
		return 0, errors.New("ONNX embedding request exceeds its admitted bounds")
	}
	var prompt struct {
		Input []string `json:"input"`
	}
	d := json.NewDecoder(bytes.NewReader(req.Prompt))
	d.DisallowUnknownFields()
	if err := d.Decode(&prompt); err != nil || len(prompt.Input) < 1 || len(prompt.Input) > 32 {
		return 0, errors.New("ONNX embeddings require one to 32 text inputs")
	}
	for _, input := range prompt.Input {
		if input == "" || len(input) > 32768 || !utf8.ValidString(input) {
			return 0, errors.New("ONNX embedding text exceeds input bounds")
		}
	}
	return len(prompt.Input), nil
}

func validateONNXEmbeddingVectors(vectors [][]float32) error {
	if len(vectors) == 0 || len(vectors) > 32 || len(vectors[0]) == 0 || len(vectors[0]) > 4096 || len(vectors)*len(vectors[0]) > maxONNXEmbeddingValues {
		return errors.New("ONNX embedding result exceeds vector bounds")
	}
	for _, vector := range vectors {
		if len(vector) != len(vectors[0]) {
			return errors.New("ONNX embedding result has inconsistent dimensions")
		}
		var norm float64
		for _, value := range vector {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return errors.New("ONNX embedding result is not finite")
			}
			norm += float64(value) * float64(value)
		}
		if norm == 0 {
			return errors.New("ONNX embedding result has zero norm")
		}
	}
	return nil
}
