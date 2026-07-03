package file_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/smykla-skalski/klaudiush/internal/validators/file"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
)

var _ = Describe("AICommentValidator broadened coverage", func() {
	var (
		v   *file.AICommentValidator
		ctx *hook.Context
	)

	BeforeEach(func() {
		v = file.NewAICommentValidator(logger.NewNoOpLogger(), nil, nil)
		ctx = &hook.Context{
			EventType: hook.EventTypePreToolUse,
			ToolName:  hook.ToolTypeWrite,
		}
	})

	DescribeTable(
		"flags comments that open with a restating verb",
		func(content string) {
			ctx.ToolInput.Content = content
			result := v.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeFalse())
		},
		Entry("configure", "// Configure the http client\ncli := newClient()"),
		Entry("handle", "// Handle the incoming request\nserve(r)"),
		Entry("parse", "// Parse the json payload\np := parse(b)"),
		Entry("increment without article", "// Increment counter\ncounter++"),
		Entry("append", "// Append the item\nlist = append(list, x)"),
		Entry("validate", "// Validate the input\ncheckInput(in)"),
		Entry("iterate", "// Iterate over rows\nfor range rows {\n}"),
		Entry("send", "// Send the response\nwrite(b)"),
	)

	DescribeTable(
		"allows comments that are not filler",
		func(content string) {
			ctx.ToolInput.Content = content
			result := v.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeTrue())
		},
		Entry("TODO marker", "// TODO: return the cached value\nreturn nil"),
		Entry("FIXME marker", "// FIXME handle the retry path\nx := 1"),
		Entry("exported Go func doc",
			"// Returns the user for the given id.\nfunc GetUser(id string) *User { return nil }"),
		Entry("exported JS function",
			"// Creates a new widget instance.\nexport function makeWidget() {}"),
		Entry("why comment",
			"// Guard against nil to avoid a shutdown panic.\nif cli == nil {\n}"),
	)

	DescribeTable(
		"still flags filler that is not a doc comment",
		func(content string) {
			ctx.ToolInput.Content = content
			result := v.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeFalse())
		},
		Entry("unexported func",
			"// Returns the user for the given id.\nfunc getUser(id string) *User { return nil }"),
		Entry("blank line breaks doc association",
			"// Returns the user.\n\nfunc GetUser() {\n}"),
	)
	DescribeTable(
		"allows // or # inside string and URL literals",
		func(content string) {
			ctx.ToolInput.Content = content
			result := v.Validate(context.Background(), ctx)
			Expect(result.Passed).To(BeTrue())
		},
		Entry("https url with verb after slashes",
			`endpoint := "https://set.example.com/v1"`),
		Entry("http url with verb",
			`u := "http://config.example.com/handle"`),
		Entry("hash inside string literal",
			`color := "#set0aa"`),
	)
})
