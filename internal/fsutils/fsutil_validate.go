package fsutil

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateDocID performs a conservative validation suitable for using id as a single path segment.
// It does not enforce canonical UUID formatting; callers can add a stricter check if desired.
func ValidateDocID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("empty document id")
	}
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return fmt.Errorf("invalid document id: %q", id)
	}
	return nil
}

// ValidateMediaName ensures media names are base names (no separators/traversal).
func ValidateMediaName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("empty media name")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid media name: %q", name)
	}
	return nil
}
