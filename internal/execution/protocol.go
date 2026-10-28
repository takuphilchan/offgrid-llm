package execution

import (
	"bytes"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

const MaxFrameBytes = 1024 * 1024

var ErrProtocol = errors.New("execution_protocol_invalid")

type Request struct {
	Version     int             `json:"version"`
	ID          string          `json:"id"`
	OperationID string          `json:"operation_id"`
	Binding     Binding         `json:"binding"`
	Token       string          `json:"token"`
	Operation   string          `json:"operation"`
	Arguments   json.RawMessage `json:"arguments"`
	// Approved is set by the authenticated supervisor for one exact invocation,
	// never decoded from model arguments. It is not reusable session authority.
	Approved bool `json:"approved,omitempty"`
}

type Response struct {
	Version int      `json:"version"`
	ID      string   `json:"id"`
	Receipt *Receipt `json:"receipt,omitempty"`
	Error   string   `json:"error,omitempty"`
}

func decodeStrict(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' || jsonwire.Validate(data) != nil {
		return ErrProtocol
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil || d.Decode(new(any)) != io.EOF {
		return ErrProtocol
	}
	return nil
}

func ReadFrame[T any](reader io.Reader, target *T) error {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > MaxFrameBytes {
		return ErrProtocol
	}
	body := make([]byte, int(size))
	if _, err := io.ReadFull(reader, body); err != nil {
		return err
	}
	// Each frame is a complete envelope. Missing optional fields must not keep
	// a prior frame's error, receipt, credential or binding when callers reuse
	// their destination variable.
	var decoded T
	if err := decodeStrict(body, &decoded); err != nil {
		return err
	}
	*target = decoded
	return nil
}

func WriteFrame(writer io.Writer, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(body) == 0 || len(body) > MaxFrameBytes {
		return ErrLimit
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(body)))
	if _, err = io.Copy(writer, bytes.NewReader(header[:])); err != nil {
		return err
	}
	_, err = io.Copy(writer, bytes.NewReader(body))
	return err
}

func (r Request) validate(token string) error {
	if r.Version != ProtocolVersion || !validIdentity(r.ID) || !validIdentity(r.OperationID) || len(token) != 64 || subtle.ConstantTimeCompare([]byte(r.Token), []byte(token)) != 1 {
		return ErrAuthority
	}
	if len(r.Arguments) == 0 || jsonwire.Validate(r.Arguments) != nil {
		return ErrProtocol
	}
	return nil
}

// Payload binds intent to exact structured arguments and the job/session grant,
// but does not persist the private transport credential or transient request ID.
func (r Request) Payload() []byte {
	data, _ := json.Marshal(struct {
		Version   int
		Binding   Binding
		Operation string
		Arguments json.RawMessage
		Approved  bool
	}{r.Version, r.Binding, r.Operation, r.Arguments, r.Approved})
	return data
}
