package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode"
)

// protect caps the body before routing to a mutation handler.
func decodeStrict(r *http.Request, value any) error {
	reader := json.NewDecoder(r.Body)
	var raw json.RawMessage
	if err := reader.Decode(&raw); err != nil {
		return err
	}
	if len(raw) == 0 || raw[0] != '{' {
		return errors.New("body must contain one JSON object")
	}
	var extra json.RawMessage
	if err := reader.Decode(&extra); err != io.EOF {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			return err
		}
		return errors.New("body must contain one JSON object")
	}
	keys := json.NewDecoder(bytes.NewReader(raw))
	keys.UseNumber()
	if err := checkJSONKeys(keys, 0); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func checkJSONKeys(reader *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("JSON nesting exceeds 64 levels")
	}
	token, err := reader.Token()
	if err != nil || (token != json.Delim('{') && token != json.Delim('[')) {
		return err
	}
	if token == json.Delim('{') {
		keys := make(map[string]struct{})
		for reader.More() {
			key, err := reader.Token()
			if err != nil {
				return err
			}
			name := strings.Map(func(r rune) rune {
				// Match encoding/json's case-insensitive field lookup.
				for {
					next := unicode.SimpleFold(r)
					if next <= r {
						return next
					}
					r = next
				}
			}, key.(string))
			if _, exists := keys[name]; exists {
				return errors.New("duplicate JSON field")
			}
			keys[name] = struct{}{}
			if err := checkJSONKeys(reader, depth+1); err != nil {
				return err
			}
		}
	} else {
		for reader.More() {
			if err := checkJSONKeys(reader, depth+1); err != nil {
				return err
			}
		}
	}
	_, err = reader.Token()
	return err
}
