// Package validation provides config-specific validation implementations
// This extends the generic validation helpers with config type interfaces.
package validation

import (
	"fmt"
	"time"
)

// ServiceIDProvider implements IDProvider for Service
type ServiceIDProvider interface {
	GetID() string
	GetName() string
	GetGroup() string
	GetHealthInterval() time.Duration
	GetHealthTimeout() time.Duration
}

// GroupIDProvider implements IDProvider for Group
type GroupIDProvider interface {
	GetID() string
}

// ValidateServices validates service configuration with enhanced checks
func ValidateServices[T ServiceIDProvider](services []T) error {
	// Validate unique IDs
	ids := make(map[string]bool)

	for _, service := range services {
		id := service.GetID()

		// Check ID uniqueness
		if id == "" {
			return fmt.Errorf("service ID cannot be empty")
		}
		if ids[id] {
			return fmt.Errorf("duplicate service ID: %s", id)
		}
		ids[id] = true

		// Check required name
		if service.GetName() == "" {
			return fmt.Errorf("service name cannot be empty for service: %s", id)
		}

		// Check health check timings
		if service.GetHealthInterval() < time.Second {
			return fmt.Errorf("health check interval too short for service %s: %v", id, service.GetHealthInterval())
		}

		if service.GetHealthTimeout() < 100*time.Millisecond {
			return fmt.Errorf("health check timeout too short for service %s: %v", id, service.GetHealthTimeout())
		}
	}

	return nil
}

// ValidateGroups validates group configuration
func ValidateGroups[T GroupIDProvider](groups []T) error {
	ids := make(map[string]bool)

	for _, group := range groups {
		id := group.GetID()

		if id == "" {
			return fmt.Errorf("group ID cannot be empty")
		}
		if ids[id] {
			return fmt.Errorf("duplicate group ID: %s", id)
		}
		ids[id] = true
	}

	return nil
}

// ValidateServiceGroupReferences validates that services reference valid groups
func ValidateServiceGroupReferences[S ServiceIDProvider, G GroupIDProvider](services []S, groups []G) error {
	groupIDs := make(map[string]bool)
	for _, group := range groups {
		groupIDs[group.GetID()] = true
	}

	for _, service := range services {
		if group := service.GetGroup(); group != "" && !groupIDs[group] {
			return fmt.Errorf("service %s references non-existent group: %s", service.GetID(), group)
		}
	}

	return nil
}
