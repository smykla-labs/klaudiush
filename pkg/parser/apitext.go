package parser

import (
	"regexp"
	"strings"
)

var (
	// requestCallPattern matches an SDK call that names the method and path in
	// one string, the form Octokit's request() takes:
	// octokit.request("PUT /repos/{owner}/{repo}/contents/{path}", ...)
	requestCallPattern = regexp.MustCompile(
		"(?i)request\\(\\s*[\"'`]\\s*(GET|POST|PUT|PATCH|DELETE)\\s+([^\"'`\\s]+)",
	)

	// methodCallPattern matches a client method carrying the verb in its name,
	// the form axios, requests and fetch wrappers take:
	// axios.put("https://api.github.com/repos/o/r/contents/x", body)
	methodCallPattern = regexp.MustCompile(
		"(?i)\\.(get|post|put|patch|delete)\\(\\s*[\"'`]([^\"'`]+)",
	)
)

// FindAPICallsInText finds REST calls written inside script text, so a request
// made from an inline node or python program is visible even though the shell
// command itself is only an interpreter invocation.
func FindAPICallsInText(text string) []HTTPRequest {
	requestCalls := requestCallPattern.FindAllStringSubmatch(text, -1)
	methodCalls := methodCallPattern.FindAllStringSubmatch(text, -1)

	requests := make([]HTTPRequest, 0, len(requestCalls)+len(methodCalls))

	for _, match := range requestCalls {
		requests = append(requests, HTTPRequest{
			Method:          strings.ToUpper(match[1]),
			URL:             match[2],
			ExplicitAPICall: true,
		})
	}

	for _, match := range methodCalls {
		requests = append(requests, HTTPRequest{
			Method: strings.ToUpper(match[1]),
			URL:    match[2],
		})
	}

	return requests
}

// FindCallsInText reports which of the given library call names appear in the
// text in call form, so "repos.createOrUpdateFileContents" matches only when it
// is actually invoked and not when it is merely mentioned.
func FindCallsInText(text string, calls []string) string {
	for _, call := range calls {
		if strings.Contains(text, call+"(") {
			return call
		}
	}

	return ""
}
