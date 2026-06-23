package notificationconfiguration

import (
	"testing"
)

func TestValidateS3Event(t *testing.T) {
	tests := []struct {
		name    string
		event   string
		wantErr bool
	}{
		{
			name:    "valid object created all event",
			event:   "s3:ObjectCreated:*",
			wantErr: false,
		},
		{
			name:    "valid object removed delete event",
			event:   "s3:ObjectRemoved:Delete",
			wantErr: false,
		},
		{
			name:    "valid object created put event",
			event:   "s3:ObjectCreated:Put",
			wantErr: false,
		},
		{
			name:    "invalid event",
			event:   "s3:InvalidEvent:*",
			wantErr: true,
		},
		{
			name:    "empty event",
			event:   "",
			wantErr: true,
		},
		{
			name:    "whitespace event",
			event:   "   ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateS3Event(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateS3Event() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateEvents(t *testing.T) {
	tests := []struct {
		name    string
		events  []string
		wantErr bool
	}{
		{
			name:    "valid single event",
			events:  []string{"s3:ObjectCreated:*"},
			wantErr: false,
		},
		{
			name:    "valid multiple events",
			events:  []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:*"},
			wantErr: false,
		},
		{
			name:    "invalid event in list",
			events:  []string{"s3:ObjectCreated:*", "s3:InvalidEvent:*"},
			wantErr: true,
		},
		{
			name:    "empty event list",
			events:  []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEvents(tt.events)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEvents() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilterRules(t *testing.T) {
	tests := []struct {
		name    string
		rules   []string
		wantErr bool
	}{
		{
			name:    "valid prefix rule",
			rules:   []string{"prefix"},
			wantErr: false,
		},
		{
			name:    "valid suffix rule",
			rules:   []string{"suffix"},
			wantErr: false,
		},
		{
			name:    "valid prefix and suffix",
			rules:   []string{"prefix", "suffix"},
			wantErr: false,
		},
		{
			name:    "invalid rule name",
			rules:   []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "empty rules",
			rules:   []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilterRules(tt.rules)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilterRules() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
