package execution

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"strings"
	"unicode/utf8"
)

const InputWriteTool = "workspace_write_from_input"
const InputWriteVersion = 1
const MaxInlineWriteBytes = 131072

var ErrInputSelection = errors.New("execution_input_selection_invalid")
var ErrInputVerification = errors.New("execution_input_verification_unconfirmed")

type InputWriteArguments struct {
	Path     string `json:"path"`
	Input    string `json:"input"`
	After    string `json:"after"`
	Before   string `json:"before"`
	Revision string `json:"revision,omitempty"`
}

type FileWritePayload struct {
	Path     string `json:"path"`
	Data     []byte `json:"data"`
	Revision string `json:"revision,omitempty"`
}

type InputProof struct {
	Version       int    `json:"version"`
	Input         string `json:"input"`
	InputDigest   string `json:"input_digest"`
	ContentDigest string `json:"content_digest"`
	StartByte     int    `json:"start_byte"`
	EndByte       int    `json:"end_byte"`
}

func ParseInputWrite(args json.RawMessage) (InputWriteArguments, error) {
	var a InputWriteArguments
	if decodeStrict(args, &a) != nil || !validInputSelection(a) {
		return a, ErrInputSelection
	}
	return a, nil
}

func validInputSelection(a InputWriteArguments) bool {
	if a.Path == "" || len(a.Path) > 32768 || !utf8.ValidString(a.Path) || strings.ContainsRune(a.Path, 0) || a.Input == "" || len(a.Input) > 160 || !utf8.ValidString(a.Input) {
		return false
	}
	for _, marker := range []string{a.After, a.Before} {
		if marker == "" || len(marker) > 256 || !utf8.ValidString(marker) || strings.ContainsRune(marker, 0) {
			return false
		}
	}
	if a.Revision != "" {
		decoded, err := hex.DecodeString(a.Revision)
		if err != nil || len(decoded) != 32 || a.Revision != strings.ToLower(a.Revision) {
			return false
		}
	}
	return true
}

// SelectInput preserves bytes; neither labels nor delimiters are authority.
// The caller must supply one already scoped immutable user input, never look up
// an arbitrary job, path, document or tool result on behalf of these arguments.
func SelectInput(input string, a InputWriteArguments) (FileWritePayload, InputProof, error) {
	var payload FileWritePayload
	var proof InputProof
	if !validInputSelection(a) {
		return payload, proof, ErrInputSelection
	}
	if !utf8.ValidString(input) || strings.Count(input, a.After) != 1 || strings.Count(input, a.Before) != 1 {
		return payload, proof, ErrInputSelection
	}
	start := strings.Index(input, a.After) + len(a.After)
	end := strings.Index(input, a.Before)
	if end < start || end-start > MaxInlineWriteBytes {
		return payload, proof, ErrInputSelection
	}
	data := []byte(input[start:end])
	if !utf8.Valid(data) {
		return payload, proof, ErrInputSelection
	}
	return FileWritePayload{a.Path, data, a.Revision}, InputProof{InputWriteVersion, a.Input, digest([]byte(input)), digest(data), start, end}, nil
}

func WriteReceiptMatches(request Request, receipt Receipt) bool {
	var args struct {
		Path     string  `json:"path"`
		Data     *[]byte `json:"data"`
		Revision string  `json:"revision,omitempty"`
	}
	var written WriteResult
	return request.Operation == "file_write" && receipt.State == "complete" && decodeStrict(request.Arguments, &args) == nil && args.Path != "" && args.Data != nil && jsonwire.Validate(receipt.Result) == nil && json.Unmarshal(receipt.Result, &written) == nil && written.Host == request.Binding.Host && written.Path != "" && written.Verified && written.Bytes == len(*args.Data) && written.Revision == digest(*args.Data)
}

func PresentInputWrite(ref Reference, request Request, receipt Receipt, proof InputProof) (string, error) {
	inputDigest, err := hex.DecodeString(proof.InputDigest)
	if proof.Version != InputWriteVersion || proof.Input == "" || len(proof.Input) > 160 || err != nil || len(inputDigest) != 32 || !WriteReceiptMatches(request, receipt) {
		return "", ErrInputVerification
	}
	var args FileWritePayload
	if decodeStrict(request.Arguments, &args) != nil || proof.ContentDigest != digest(args.Data) || proof.EndByte-proof.StartByte != len(args.Data) || proof.StartByte < 0 {
		return "", ErrInputVerification
	}
	view, err := PresentReceipt(ref, "file_write", receipt)
	if err != nil {
		return "", err
	}
	var envelope map[string]any
	if json.Unmarshal([]byte(view), &envelope) != nil {
		return "", ErrInputVerification
	}
	envelope["source_input"] = proof
	envelope["text"] = "Selected user-input bytes written and independently read back exactly. This verifies that selected span, not every requirement of the task."
	encoded, err := json.Marshal(envelope)
	return string(encoded), err
}
