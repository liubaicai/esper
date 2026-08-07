package esper

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func parseJSONTime(text string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02", text)
}
