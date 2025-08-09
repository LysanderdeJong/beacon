// Package validation provides reusable validation helpers for the Beacon configuration system.
// This includes generic uniqueness validators and field validation utilities.
package validation

import "fmt"

// IDProvider interface for types that have an ID field
type IDProvider interface {
	GetID() string
}

// NameProvider interface for types that have a Name field
type NameProvider interface {
	GetName() string
}

// ValidateUniqueIDs validates that all items have unique IDs
func ValidateUniqueIDs[T IDProvider](items []T, itemType string) error {
	ids := make(map[string]bool)

	for _, item := range items {
		id := item.GetID()
		if id == "" {
			return fmt.Errorf("%s ID cannot be empty", itemType)
		}

		if ids[id] {
			return fmt.Errorf("duplicate %s ID: %s", itemType, id)
		}

		ids[id] = true
	}

	return nil
}

// ValidateRequiredField validates that a field is not empty
func ValidateRequiredField(value, fieldName, contextName string) error {
	if value == "" {
		return fmt.Errorf("%s cannot be empty for %s", fieldName, contextName)
	}
	return nil
}

// ValidateFieldInSet validates that a field value is in an allowed set
func ValidateFieldInSet(value string, allowedValues map[string]bool, fieldName string) error {
	if !allowedValues[value] {
		return fmt.Errorf("invalid %s: %s", fieldName, value)
	}
	return nil
}

// ValidateReference validates that a reference exists in a set of valid IDs
func ValidateReference(refValue, refFieldName, contextName string, validIDs map[string]bool) error {
	if refValue != "" && !validIDs[refValue] {
		return fmt.Errorf("%s references non-existent %s: %s", contextName, refFieldName, refValue)
	}
	return nil
}

// CreateIDMap creates a map of IDs for reference validation
func CreateIDMap[T IDProvider](items []T) map[string]bool {
	idMap := make(map[string]bool)
	for _, item := range items {
		idMap[item.GetID()] = true
	}
	return idMap
}
