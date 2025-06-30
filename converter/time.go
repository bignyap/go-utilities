package converter

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

func ToUnixTime(t ...time.Time) int64 {

	if len(t) == 0 {
		return time.Now().Unix()
	}

	return t[0].Unix()
}

func FromUnixTime(unixTimestamp int64) (time.Time, error) {

	if unixTimestamp < 0 {
		return time.Time{}, errors.New("invalid Unix timestamp, must be non-negative")
	}

	return time.Unix(unixTimestamp, 0), nil
}

func FromUnixTime64(unixTimestamp int64) time.Time {
	return time.Unix(unixTimestamp, 0)
}

func FromUnixTime32(unixTimestamp int32) time.Time {
	return FromUnixTime64(int64(unixTimestamp))
}

type TimeOrDate struct {
	time.Time
}

// UnmarshalJSON handles JSON unmarshaling (for JSON requests)
func (t *TimeOrDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	fmt.Println("Inside UnmarshalJSON:", s)
	return t.parse(s)
}

// UnmarshalText handles text unmarshaling (for other scenarios)
func (t *TimeOrDate) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	fmt.Println("Inside UnmarshalText:", s)
	return t.parse(s)
}

func (t *TimeOrDate) parse(s string) error {
	fmt.Println("Inside parse:", s)
	if s == "" || s == "null" {
		return nil
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, s)
		if err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("invalid time format: %s", s)
}

func (t *TimeOrDate) IsZero() bool {
	return t.Time.IsZero()
}
