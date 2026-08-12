package esper

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// UUID is the portable Go representation used by typed JSON schemas for a
// JSON UUID string. It deliberately keeps the value comparable and does not
// depend on a third-party UUID package.
type UUID [16]byte

// ParseUUID parses the canonical UUID form, optionally accepting braces or a
// urn:uuid: prefix.
func ParseUUID(text string) (UUID, error) {
	var result UUID
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(strings.TrimPrefix(text, "urn:uuid:"), "URN:UUID:")
	text = strings.TrimPrefix(strings.TrimSuffix(text, "}"), "{")
	compact := strings.ReplaceAll(text, "-", "")
	if len(compact) != 32 {
		return result, fmt.Errorf("esper: invalid UUID %q", text)
	}
	decoded, err := hex.DecodeString(compact)
	if err != nil {
		return result, fmt.Errorf("esper: invalid UUID %q: %w", text, err)
	}
	copy(result[:], decoded)
	return result, nil
}

func (value UUID) String() string {
	hexValue := hex.EncodeToString(value[:])
	return hexValue[0:8] + "-" + hexValue[8:12] + "-" + hexValue[12:16] + "-" + hexValue[16:20] + "-" + hexValue[20:32]
}

func (value UUID) MarshalJSON() ([]byte, error) {
	return json.Marshal(value.String())
}

func (value *UUID) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	parsed, err := ParseUUID(text)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

// DateOnly preserves a JSON calendar date without introducing a timezone.
// It is the Go-style counterpart for Java LocalDate in typed JSON schemas.
type DateOnly string

// ParseDateOnly validates an ISO-8601 calendar date.
func ParseDateOnly(text string) (DateOnly, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(text))
	if err != nil {
		return "", fmt.Errorf("esper: invalid date %q: %w", text, err)
	}
	return DateOnly(parsed.Format("2006-01-02")), nil
}

func (value DateOnly) String() string { return string(value) }

func (value DateOnly) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(value))
}

func (value *DateOnly) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	parsed, err := ParseDateOnly(text)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

// URL is the portable Go scalar used for JSON values that must contain an
// absolute URL. Keeping it as a string-like type makes it render as the same
// JSON string that Esper exposes while still validating the URL at parse time.
type URL string

// ParseURL parses an absolute URL with a scheme and host.
func ParseURL(text string) (URL, error) {
	parsed, err := url.Parse(text)
	if err != nil {
		return "", fmt.Errorf("esper: invalid URL %q: %w", text, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("esper: invalid URL %q: absolute URL required", text)
	}
	return URL(text), nil
}

func (value URL) String() string { return string(value) }

func (value URL) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(value))
}

func (value *URL) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	parsed, err := ParseURL(text)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

// URI is the portable Go scalar counterpart for java.net.URI in typed JSON
// schemas. URI accepts relative references; malformed URI text is rejected.
type URI string

// ParseURI validates URI syntax while preserving the original text.
func ParseURI(text string) (URI, error) {
	if strings.ContainsAny(text, " \t\r\n") {
		return "", fmt.Errorf("esper: invalid URI %q: whitespace is not allowed", text)
	}
	if _, err := url.Parse(text); err != nil {
		return "", fmt.Errorf("esper: invalid URI %q: %w", text, err)
	}
	return URI(text), nil
}

func (value URI) String() string { return string(value) }

func (value URI) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(value))
}

func (value *URI) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	parsed, err := ParseURI(text)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

func parseJSONTime(text string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
		return parsed, nil
	}
	// Java's LocalDateTime has no offset, while ZonedDateTime may append a
	// region identifier in brackets. time.Time is the idiomatic Go carrier for
	// both shapes; preserve the original JSON text in the event renderer.
	parseText := text
	if bracket := strings.IndexByte(parseText, '['); bracket > 0 && strings.HasSuffix(parseText, "]") {
		parseText = parseText[:bracket]
	}
	if parsed, err := time.Parse(time.RFC3339Nano, parseText); err == nil {
		return parsed, nil
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, parseText); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("esper: invalid JSON time %q", text)
}
