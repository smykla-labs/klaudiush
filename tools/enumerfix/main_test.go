package main

import (
	"testing"
)

func TestFixEnumerFile(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "replaces fmt.Errorf with errors.Newf and updates import",
			input: `package test

import "fmt"

func foo() error {
	return fmt.Errorf("error: %s", msg)
}
`,
			expected: `package test

import "github.com/cockroachdb/errors"

func foo() error {
	return errors.Newf("error: %s", msg)
}
`,
		},
		{
			name: "keeps fmt import when fmt.Sprintf is used",
			input: `package test

import (
	"fmt"
)

func foo() (string, error) {
	s := fmt.Sprintf("value: %d", val)
	return s, fmt.Errorf("error")
}
`,
			expected: `package test

import (
	"fmt"
	"github.com/cockroachdb/errors"
)

func foo() (string, error) {
	s := fmt.Sprintf("value: %d", val)
	return s, errors.Newf("error")
}
`,
		},
		{
			name: "handles content without fmt.Errorf",
			input: `package test

import "fmt"

func foo() {
	fmt.Println("hello")
}
`,
			expected: `package test

import "github.com/cockroachdb/errors"

func foo() {
	fmt.Println("hello")
}
`,
		},
		{
			name: "does not duplicate errors import",
			input: `package test

import (
	"fmt"
	"github.com/cockroachdb/errors"
)

func foo() error {
	s := fmt.Sprintf("value")
	return fmt.Errorf("error")
}
`,
			expected: `package test

import (
	"fmt"
	"github.com/cockroachdb/errors"
)

func foo() error {
	s := fmt.Sprintf("value")
	return errors.Newf("error")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixEnumerFile([]byte(tt.input))

			if string(result) != tt.expected {
				t.Errorf("fixEnumerFile() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestAddErrorsImport(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "adds errors import to existing import block",
			input: `package test

import (
	"fmt"
)
`,
			expected: `package test

import (
	"fmt"
	"github.com/cockroachdb/errors"
)
`,
		},
		{
			name: "does not add duplicate errors import",
			input: `package test

import (
	"fmt"
	"github.com/cockroachdb/errors"
)
`,
			expected: `package test

import (
	"fmt"
	"github.com/cockroachdb/errors"
)
`,
		},
		{
			name: "returns unchanged content without import block",
			input: `package test

var x = 1
`,
			expected: `package test

var x = 1
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addErrorsImport(tt.input)

			if result != tt.expected {
				t.Errorf("addErrorsImport() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestReplaceImport(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		oldImport string
		newImport string
		expected  string
	}{
		{
			name:      "replaces single-line import",
			input:     `import "fmt"`,
			oldImport: `"fmt"`,
			newImport: `"github.com/cockroachdb/errors"`,
			expected:  `import "github.com/cockroachdb/errors"`,
		},
		{
			name: "replaces import in multi-line block",
			input: `import (
	"fmt"
	"strings"
)`,
			oldImport: `"fmt"`,
			newImport: `"github.com/cockroachdb/errors"`,
			expected: `import (
	"github.com/cockroachdb/errors"
	"strings"
)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceImport(tt.input, tt.oldImport, tt.newImport)

			if result != tt.expected {
				t.Errorf("replaceImport() = %q, want %q", result, tt.expected)
			}
		})
	}
}
