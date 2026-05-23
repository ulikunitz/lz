// Package jt defines the Size type to handle size parameters in JSON structures
// in a more user-friendly way. 
package jt

import (
	"fmt"
	"regexp"
	"strconv"
	"sync"
)

// Size is a specific type for handling data size parameters. It shortens the
// string representation. For instance 8 MiB are represented as "8M", 16 KiB as "16K", 2 GiB
// as "2G". It is also possible to use plain numbers for it.
type Size int

// String returns the string representation of the size. It uses K, M, and G as suffixes for
// KiB, MiB, and GiB, respectively.
func (s Size) String() string {
	switch {
	case s == 0:
		return "0"
	case s%(1<<30) == 0:
		return fmt.Sprintf("%dG", s/(1<<30))
	case s%(1<<20) == 0:
		return fmt.Sprintf("%dM", s/(1<<20))
	case s%(1<<10) == 0:
		return fmt.Sprintf("%dK", s/(1<<10))
	default:
		return fmt.Sprintf("%d", s)
	}
}

// MarshalJSON returns the string representation of the size as byte slice. It
// is used by the JSON encoder.
func (s Size) MarshalJSON() ([]byte, error) {
	a := s.String()
	if c := a[len(a)-1]; c == 'K' || c == 'M' || c == 'G' {
		return []byte("\"" + a + "\""), nil
	}
	return []byte(a), nil
}

var sizeRegexp = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^(\d+)([KMG]?)$`)
})

// parseSize parses the string representation of the byteSize type.
func parseSize(s string) (size Size, err error) {
	const msg = "lz: invalid size %q; must be in format <number>[K|M|G]?"
	m := sizeRegexp().FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf(msg, s)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf(msg, s)
	}
	switch m[2] {
	case "K":
		n *= 1 << 10
	case "M":
		n *= 1 << 20
	case "G":
		n *= 1 << 30
	}
	return Size(n), nil
}

// UnmarshalText parses the string representation of the size and sets the value
// of s. It is used by the JSON decoder.
func (s *Size) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		if len(data) < 2 || data[len(data)-1] != '"' {
			return fmt.Errorf("lz: invalid size %q", string(data))
		}
		data = data[1 : len(data)-1]

	}
	var err error
	*s, err = parseSize(string(data))
	return err
}
