package notificationconfiguration

import (
	"fmt"
	"strings"
)

// Valid S3 event types for bucket notifications
var validS3Events = map[string]bool{
	// Object creation events
	"s3:ObjectCreated:*":                       true,
	"s3:ObjectCreated:Put":                     true,
	"s3:ObjectCreated:Post":                    true,
	"s3:ObjectCreated:Copy":                    true,
	"s3:ObjectCreated:CompleteMultipartUpload": true,

	// Object removal events
	"s3:ObjectRemoved:*":                   true,
	"s3:ObjectRemoved:Delete":              true,
	"s3:ObjectRemoved:DeleteMarkerCreated": true,

	// Object restoration events
	"s3:ObjectRestore:*":         true,
	"s3:ObjectRestore:Post":      true,
	"s3:ObjectRestore:Completed": true,

	// Object tagging events
	"s3:ObjectTagging:*":      true,
	"s3:ObjectTagging:Put":    true,
	"s3:ObjectTagging:Delete": true,

	// Object ACL events
	"s3:ObjectAcl:*":   true,
	"s3:ObjectAcl:Put": true,

	// Replication events
	"s3:Replication:*":                                 true,
	"s3:Replication:OperationFailedReplication":        true,
	"s3:Replication:OperationNotTracked":               true,
	"s3:Replication:OperationMissedThreshold":          true,
	"s3:Replication:OperationReplicatedAfterThreshold": true,
}

// ValidateS3Event checks if the given event is a valid S3 notification event
func ValidateS3Event(event string) error {
	if event == "" {
		return fmt.Errorf("event cannot be empty")
	}

	// Trim whitespace
	event = strings.TrimSpace(event)

	if !validS3Events[event] {
		return fmt.Errorf("invalid S3 event: %s", event)
	}

	return nil
}

// ValidateEvents checks if all events in the slice are valid S3 notification events
func ValidateEvents(events []string) error {
	if len(events) == 0 {
		return fmt.Errorf("at least one event is required")
	}

	for _, event := range events {
		if err := ValidateS3Event(event); err != nil {
			return err
		}
	}

	return nil
}

// ValidateFilterRules validates filter rule names
func ValidateFilterRules(rules []string) error {
	validRuleNames := map[string]bool{
		"prefix": true,
		"suffix": true,
	}

	for _, rule := range rules {
		if !validRuleNames[rule] {
			return fmt.Errorf("invalid filter rule name: %s (must be 'prefix' or 'suffix')", rule)
		}
	}

	return nil
}
