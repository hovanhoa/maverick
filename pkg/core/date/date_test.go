package date

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(t *testing.T, result *time.Time)
	}{
		{
			name:    "RFC1123Z format",
			input:   "Mon, 02 Jan 2006 15:04:05 -0700",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2006, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 2, result.Day())
			},
		},
		{
			name:    "RFC3339 format",
			input:   "2024-03-15T10:30:00Z",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.March, result.Month())
				assert.Equal(t, 15, result.Day())
			},
		},
		{
			name:    "RFC3339Nano format",
			input:   "2024-03-15T10:30:00.123456789Z",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.March, result.Month())
				assert.Equal(t, 15, result.Day())
			},
		},
		{
			name:    "Date with single digit day and timezone",
			input:   " 2 Jan 2006 15:04:05 -0700",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2006, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 2, result.Day())
			},
		},
		{
			name:    "Date with two digit day and timezone",
			input:   "02 Jan 2006 15:04:05 -0700",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2006, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 2, result.Day())
			},
		},
		{
			name:    "Friendly format with comma - Mon, Jan 2, 2006 at 3:04 PM",
			input:   "Mon, Jan 2, 2006 at 3:04 PM",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2006, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 2, result.Day())
				assert.Equal(t, 15, result.Hour())
				assert.Equal(t, 4, result.Minute())
			},
		},
		{
			name:    "Friendly format with two digit day - Mon, Jan 02, 2006 at 3:04 PM",
			input:   "Mon, Jan 02, 2006 at 3:04 PM",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2006, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 2, result.Day())
			},
		},
		{
			name:    "Friendly format with zero padded time - Mon, Jan 02, 2006 at 03:04 PM",
			input:   "Mon, Jan 02, 2006 at 03:04 PM",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2006, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 2, result.Day())
			},
		},
		{
			name:    "Simple datetime format",
			input:   "2024-03-15 14:30:00",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.March, result.Month())
				assert.Equal(t, 15, result.Day())
				assert.Equal(t, 14, result.Hour())
				assert.Equal(t, 30, result.Minute())
			},
		},
		{
			name:    "ISO datetime with T separator",
			input:   "2024-03-15T14:30:00",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.March, result.Month())
				assert.Equal(t, 15, result.Day())
			},
		},
		{
			name:    "Date only format",
			input:   "2024-03-15",
			wantErr: false,
			validate: func(t *testing.T, result *time.Time) {
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.March, result.Month())
				assert.Equal(t, 15, result.Day())
			},
		},
		{
			name:    "Invalid format returns error",
			input:   "not a date",
			wantErr: true,
		},
		{
			name:    "Empty string returns error",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Partial date returns error",
			input:   "2024-03",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDate(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			tt.validate(t, result)
		})
	}
}
