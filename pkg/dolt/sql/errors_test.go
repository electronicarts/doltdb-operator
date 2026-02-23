// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package sql

import (
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestIsReadOnlyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "MySQL error 1105 (read-only)",
			err:  &mysql.MySQLError{Number: 1105, Message: "database server is set to read only mode"},
			want: true,
		},
		{
			name: "MySQL error 1049 (unknown database)",
			err:  &mysql.MySQLError{Number: 1049, Message: "Unknown database"},
			want: false,
		},
		{
			name: "MySQL error 1045 (access denied)",
			err:  &mysql.MySQLError{Number: 1045, Message: "Access denied"},
			want: false,
		},
		{
			name: "non-MySQL error",
			err:  fmt.Errorf("connection refused"),
			want: false,
		},
		{
			name: "wrapped MySQL error 1105",
			err:  fmt.Errorf("transition failed: %w", &mysql.MySQLError{Number: 1105, Message: "read only"}),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsReadOnlyError(tt.err); got != tt.want {
				t.Errorf("IsReadOnlyError() = %v, want %v", got, tt.want)
			}
		})
	}
}
