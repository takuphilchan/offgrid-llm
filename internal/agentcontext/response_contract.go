package agentcontext

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"unicode/utf8"
)

// ResponseContract is inert reviewed data, never a program or tool grant.
// Patterns use Go's bounded RE2 implementation and match the entire answer.
type ResponseContract struct {
	Version  int    `json:"version"`
	Pattern  string `json:"pattern"`
	MaxBytes int    `json:"max_bytes"`
}

func (c ResponseContract) Validate() error {
	if c.Version != 1 || len(c.Pattern) == 0 || len(c.Pattern) > 2048 || !utf8.ValidString(c.Pattern) || c.MaxBytes < 1 || c.MaxBytes > 32768 {
		return errors.New("invalid skill response contract")
	}
	_, err := regexp.Compile(`\A(?:` + c.Pattern + `)\z`)
	if err != nil {
		return errors.New("invalid skill response pattern")
	}
	return nil
}

func (c ResponseContract) Matches(answer string) bool {
	if c.Validate() != nil || !utf8.ValidString(answer) || len(answer) > c.MaxBytes {
		return false
	}
	re, _ := regexp.Compile(`\A(?:` + c.Pattern + `)\z`)
	return re.MatchString(answer)
}

func parseResponseContract(value any) (*ResponseContract, error) {
	raw, err := json.Marshal(value)
	if err != nil || bytes.Equal(raw, []byte("null")) {
		return nil, ErrSkill
	}
	var contract ResponseContract
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&contract) != nil || contract.Validate() != nil {
		return nil, ErrSkill
	}
	return &contract, nil
}
