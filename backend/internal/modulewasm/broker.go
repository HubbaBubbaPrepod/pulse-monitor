package modulewasm

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

type Request struct {
	Method string
	Params map[string]json.RawMessage
}

// DecodeRequest rejects ambiguous objects before interpreting authority-bearing
// selectors. It does not accept case-folded field aliases or duplicate members.
func DecodeRequest(raw []byte) (Request, error) {
	var request Request
	if len(raw) == 0 || len(raw) > MaxRequestBytes {
		return request, errors.New("Invalid broker request")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := uniqueValue(decoder, 0); err != nil {
		return request, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return request, errors.New("Expected one broker request")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return request, errors.New("Expected a broker object")
	}
	for key := range fields {
		if key != "method" && key != "params" {
			return request, errors.New("Unknown broker field")
		}
	}
	if json.Unmarshal(fields["method"], &request.Method) != nil || len(request.Method) == 0 || len(request.Method) > 64 {
		return request, errors.New("Invalid broker method")
	}
	request.Params = map[string]json.RawMessage{}
	if value, ok := fields["params"]; ok {
		if json.Unmarshal(value, &request.Params) != nil || request.Params == nil {
			return request, errors.New("Invalid broker parameters")
		}
	}
	return request, nil
}

func uniqueValue(decoder *json.Decoder, depth int) error {
	if depth > 8 {
		return errors.New("Broker request nesting exceeds limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delimiter != '{' && delimiter != '[' {
		return errors.New("Invalid JSON delimiter")
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			name, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := name.(string)
			if !ok || seen[key] {
				return errors.New("Duplicate broker field")
			}
			seen[key] = true
		}
		if err := uniqueValue(decoder, depth+1); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

func Reply(status string, data any, message string) json.RawMessage {
	if status != "ok" && status != "error" && status != "unknown" {
		status = "unknown"
	}
	if len(message) > 256 {
		message = "Host request failed"
	}
	raw, err := json.Marshal(struct {
		OK     bool   `json:"ok"`
		Status string `json:"status"`
		Data   any    `json:"data,omitempty"`
		Error  string `json:"error,omitempty"`
	}{status == "ok", status, data, message})
	if err != nil || len(raw) > MaxOutputBytes {
		return json.RawMessage(`{"ok":false,"status":"unknown","error":"Host response unavailable"}`)
	}
	return raw
}

// PublicFields copies invocation values while withholding both classified
// secrets and reserved broker settings, independent of a module's field types.
func PublicFields(values map[string]any, secrets []string) map[string]any {
	hidden := map[string]bool{"telegramToken": true, "telegramChatId": true, "serverId": true, "socket": true}
	for _, key := range secrets {
		hidden[key] = true
	}
	public := map[string]any{}
	for key, value := range values {
		if !hidden[key] {
			public[key] = value
		}
	}
	return public
}
