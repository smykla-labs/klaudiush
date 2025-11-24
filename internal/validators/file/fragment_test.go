package file_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-labs/klaudiush/internal/validators/file"
	"github.com/smykla-labs/klaudiush/pkg/logger"
)

var _ = Describe("ExtractEditFragment", func() {
	var log logger.Logger

	BeforeEach(func() {
		log = logger.NewNoOpLogger()
	})

	Context("single-line edits", func() {
		It("extracts fragment with full context", func() {
			content := `line 1
line 2
line 3
line 4 to change
line 5
line 6
line 7`

			result := file.ExtractEditFragment(
				content,
				"line 4 to change",
				"line 4 changed",
				2,
				log,
			)

			expected := `line 2
line 3
line 4 changed
line 5
line 6`

			Expect(result).To(Equal(expected))
		})

		It("handles edits at the beginning with limited context before", func() {
			content := `line 1 to change
line 2
line 3
line 4
line 5`

			result := file.ExtractEditFragment(
				content,
				"line 1 to change",
				"line 1 changed",
				2,
				log,
			)

			expected := `line 1 changed
line 2
line 3`

			Expect(result).To(Equal(expected))
		})

		It("handles edits at the end with limited context after", func() {
			content := `line 1
line 2
line 3
line 4
line 5 to change`

			result := file.ExtractEditFragment(
				content,
				"line 5 to change",
				"line 5 changed",
				2,
				log,
			)

			expected := `line 3
line 4
line 5 changed`

			Expect(result).To(Equal(expected))
		})

		It("handles single line file", func() {
			content := `only line to change`

			result := file.ExtractEditFragment(
				content,
				"only line to change",
				"only line changed",
				2,
				log,
			)

			expected := `only line changed`

			Expect(result).To(Equal(expected))
		})

		It("handles partial line replacement", func() {
			content := `line 1
function foo() {
  return bar
}
line 5`

			result := file.ExtractEditFragment(
				content,
				"bar",
				"baz",
				2,
				log,
			)

			// Includes 2 lines before ("line 1" and "function foo() {")
			// and 2 lines after ("}" and "line 5")
			expected := `line 1
function foo() {
  return baz
}
line 5`

			Expect(result).To(Equal(expected))
		})
	})

	Context("multi-line edits", func() {
		It("extracts fragment for multi-line replacement", func() {
			content := `line 1
line 2
old line A
old line B
old line C
line 6
line 7`

			result := file.ExtractEditFragment(
				content,
				`old line A
old line B
old line C`,
				`new line A
new line B`,
				2,
				log,
			)

			expected := `line 1
line 2
new line A
new line B
line 6
line 7`

			Expect(result).To(Equal(expected))
		})

		It("handles multi-line edit at file beginning", func() {
			content := `old line 1
old line 2
line 3
line 4
line 5`

			result := file.ExtractEditFragment(
				content,
				`old line 1
old line 2`,
				`new line 1
new line 2`,
				2,
				log,
			)

			expected := `new line 1
new line 2
line 3
line 4`

			Expect(result).To(Equal(expected))
		})

		It("handles multi-line edit at file end", func() {
			content := `line 1
line 2
line 3
old line 4
old line 5`

			result := file.ExtractEditFragment(
				content,
				`old line 4
old line 5`,
				`new line 4
new line 5`,
				2,
				log,
			)

			expected := `line 2
line 3
new line 4
new line 5`

			Expect(result).To(Equal(expected))
		})
	})

	Context("context lines with function boundaries", func() {
		It("includes partial functions in context", func() {
			content := `func before() {
  doSomething()
}

func target() {
  old code
}

func after() {
  doOtherThing()
}`

			result := file.ExtractEditFragment(
				content,
				"  old code",
				"  new code",
				2,
				log,
			)

			// Includes 2 lines before and 2 lines after the changed line
			expected := `
func target() {
  new code
}
`

			Expect(result).To(Equal(expected))
		})

		It("handles edits within nested structures", func() {
			content := `type Config struct {
  Name string
  Value int
  OldField string
  Extra bool
}`

			result := file.ExtractEditFragment(
				content,
				"  OldField string",
				"  NewField string",
				2,
				log,
			)

			// Includes 2 lines before and 2 lines after
			expected := `  Name string
  Value int
  NewField string
  Extra bool
}`

			Expect(result).To(Equal(expected))
		})
	})

	Context("edge cases", func() {
		It("returns empty string when old string not found", func() {
			content := `line 1
line 2
line 3`

			result := file.ExtractEditFragment(
				content,
				"non-existent",
				"replacement",
				2,
				log,
			)

			Expect(result).To(BeEmpty())
		})

		It("handles empty lines in context", func() {
			content := `line 1

line 3
old content
line 5

line 7`

			result := file.ExtractEditFragment(
				content,
				"old content",
				"new content",
				2,
				log,
			)

			expected := `
line 3
new content
line 5
`

			Expect(result).To(Equal(expected))
		})

		It("handles indented content", func() {
			content := "  line 1\n    line 2\n      old line\n    4\n  line"

			result := file.ExtractEditFragment(
				content,
				"      old line",
				"      new line",
				2,
				log,
			)

			// Includes 2 lines before and 2 lines after
			expected := "  line 1\n    line 2\n      new line\n    4\n  line"

			Expect(result).To(Equal(expected))
		})

		It("handles content with special characters", func() {
			content := `line 1
line 2: old $value
line 3`

			result := file.ExtractEditFragment(
				content,
				"line 2: old $value",
				"line 2: new $value",
				1,
				log,
			)

			expected := `line 1
line 2: new $value
line 3`

			Expect(result).To(Equal(expected))
		})

		It("handles zero context lines", func() {
			content := "line 1\nline 2\nold line\n4\nline"

			result := file.ExtractEditFragment(
				content,
				"old line",
				"new line",
				0,
				log,
			)

			expected := `new line`

			Expect(result).To(Equal(expected))
		})

		It("handles context larger than file", func() {
			content := "line 1\nold line\n"

			result := file.ExtractEditFragment(
				content,
				"old line",
				"new line",
				10,
				log,
			)

			expected := "line 1\nnew line\n"

			Expect(result).To(Equal(expected))
		})
	})

	Context("markdown-specific scenarios", func() {
		It("includes context for heading spacing validation", func() {
			content := `# Heading 1

Some text
## Old Heading
More text

# Heading 2`

			result := file.ExtractEditFragment(
				content,
				"## Old Heading",
				"## New Heading",
				2,
				log,
			)

			expected := `
Some text
## New Heading
More text
`

			Expect(result).To(Equal(expected))
		})

		It("includes context for list validation", func() {
			content := `- Item 1
- Item 2
- Old item
- Item 4
- Item 5`

			result := file.ExtractEditFragment(
				content,
				"- Old item",
				"- New item",
				2,
				log,
			)

			expected := `- Item 1
- Item 2
- New item
- Item 4
- Item 5`

			Expect(result).To(Equal(expected))
		})
	})

	Context("shell script scenarios", func() {
		It("includes context for function validation", func() {
			content := `#!/bin/bash

function before() {
  echo "before"
}

function target() {
  old_command
}

function after() {
  echo "after"
}`

			result := file.ExtractEditFragment(
				content,
				"  old_command",
				"  new_command",
				2,
				log,
			)

			// Includes 2 lines before and 2 lines after
			expected := `
function target() {
  new_command
}
`

			Expect(result).To(Equal(expected))
		})

		It("includes context for variable assignment", func() {
			content := `VAR1="value1"
VAR2="value2"
OLD_VAR="old"
VAR3="value3"
VAR4="value4"`

			result := file.ExtractEditFragment(
				content,
				`OLD_VAR="old"`,
				`NEW_VAR="new"`,
				2,
				log,
			)

			expected := `VAR1="value1"
VAR2="value2"
NEW_VAR="new"
VAR3="value3"
VAR4="value4"`

			Expect(result).To(Equal(expected))
		})
	})
})
