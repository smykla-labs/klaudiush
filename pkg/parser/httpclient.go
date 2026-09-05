package parser

import (
	"strings"
)

// HTTPRequest is an HTTP request found in a shell command, from a client such
// as curl or wget rather than from gh.
type HTTPRequest struct {
	// Tool is the command that sends the request ("curl", "wget", ...).
	Tool string

	// Method is the effective HTTP method, upper-case.
	Method string

	// URL is the request target exactly as written.
	URL string

	// Body holds inline request body text, used to spot a GraphQL mutation.
	Body string

	// BodyFile is a body read from a file, from "-d @file" or "--body-file".
	BodyFile string

	// WorkingDirectory is the effective directory of the command.
	WorkingDirectory string

	// Location is the position of the command in the source.
	Location Location

	// ExplicitAPICall marks a call whose syntax names an API client, such as
	// Octokit's request("PUT /repos/..."). Only such a call may be judged on a
	// path with no host, since any other framework's ".put('/repos/...')" is
	// just as likely to be a local route.
	ExplicitAPICall bool
}

// IsWriteMethod reports whether the request changes server state.
func (r *HTTPRequest) IsWriteMethod() bool {
	return writeMethods[r.Method]
}

// flagRole is what a client flag's value contributes to the request.
type flagRole uint8

const (
	roleNone flagRole = iota
	roleMethod
	roleBody
	roleBodyFile
	roleURL
)

// clientFlag describes one flag of an HTTP client command.
type clientFlag struct {
	// name is the flag as written, including its dashes.
	name string

	// takesValue marks a flag that consumes the following argument when no
	// value is attached, so its value cannot be mistaken for the URL.
	takesValue bool

	// role is what the flag's value means to the request.
	role flagRole

	// implies is the method the flag's presence selects when none is explicit.
	implies string
}

// httpClientSpec describes how one client carries its method, URL and body.
type httpClientSpec struct {
	flags []clientFlag

	// positionalMethod allows a bare verb positional to set the method, the
	// form httpie and xh use.
	positionalMethod bool
}

// find returns the descriptor for a flag name.
func (s *httpClientSpec) find(name string) (clientFlag, bool) {
	for _, flag := range s.flags {
		if flag.name == name {
			return flag, true
		}
	}

	return clientFlag{}, false
}

// curlFlags lists the curl flags that matter, plus the value-taking flags that
// would otherwise have their value read as the URL.
var curlFlags = []clientFlag{
	{name: "-X", takesValue: true, role: roleMethod},
	{name: "--request", takesValue: true, role: roleMethod},
	{name: "--url", takesValue: true, role: roleURL},
	{name: "-d", takesValue: true, role: roleBody, implies: methodPOST},
	{name: "--data", takesValue: true, role: roleBody, implies: methodPOST},
	{name: "--data-raw", takesValue: true, role: roleBody, implies: methodPOST},
	{name: "--data-binary", takesValue: true, role: roleBody, implies: methodPOST},
	{name: "--data-urlencode", takesValue: true, role: roleBody, implies: methodPOST},
	{name: "--json", takesValue: true, role: roleBody, implies: methodPOST},
	{name: flagFieldShort, takesValue: true, role: roleBody, implies: methodPOST},
	{name: "--form", takesValue: true, role: roleBody, implies: methodPOST},
	{name: "-T", takesValue: true, role: roleBodyFile, implies: methodPUT},
	{name: "--upload-file", takesValue: true, role: roleBodyFile, implies: methodPUT},
	{name: "-I", implies: methodHEAD},
	{name: "--head", implies: methodHEAD},
	{name: "-H", takesValue: true},
	{name: flagHeaderLong, takesValue: true},
	{name: "-o", takesValue: true},
	{name: "--output", takesValue: true},
	{name: "-u", takesValue: true},
	{name: "--user", takesValue: true},
	{name: "-A", takesValue: true},
	{name: "--user-agent", takesValue: true},
	{name: "-e", takesValue: true},
	{name: "--referer", takesValue: true},
	{name: flagBodyShort, takesValue: true},
	{name: "--cookie", takesValue: true},
	{name: "-c", takesValue: true},
	{name: "--cookie-jar", takesValue: true},
	{name: flagMergeShort, takesValue: true},
	{name: "--max-time", takesValue: true},
	{name: "-w", takesValue: true},
	{name: "--write-out", takesValue: true},
	{name: "--connect-timeout", takesValue: true},
	{name: "--retry", takesValue: true},
	{name: "--resolve", takesValue: true},
	{name: "--cacert", takesValue: true},
	{name: "--cert", takesValue: true},
	{name: "--key", takesValue: true},
	{name: "--proxy", takesValue: true},
}

// wgetFlags lists the wget flags that carry the method or a body.
var wgetFlags = []clientFlag{
	{name: "--method", takesValue: true, role: roleMethod},
	{name: "--post-data", takesValue: true, role: roleBody, implies: methodPOST},
	{name: "--post-file", takesValue: true, role: roleBodyFile, implies: methodPOST},
	{name: "--body-data", takesValue: true, role: roleBody},
	{name: "--body-file", takesValue: true, role: roleBodyFile},
	{name: flagHeaderLong, takesValue: true},
	{name: "-O", takesValue: true},
	{name: "--output-document", takesValue: true},
}

// httpieFlags lists the httpie and xh flags whose value could be read as the
// URL. Both take the method as a bare positional verb instead of a flag.
var httpieFlags = []clientFlag{
	{name: "--raw", takesValue: true, role: roleBody},
	{name: "-a", takesValue: true},
	{name: "--auth", takesValue: true},
	{name: "-A", takesValue: true},
	{name: "--auth-type", takesValue: true},
	{name: "-o", takesValue: true},
	{name: "--output", takesValue: true},
	{name: "--proxy", takesValue: true},
}

var httpClientSpecs = map[string]httpClientSpec{
	"curl":  {flags: curlFlags},
	"wget":  {flags: wgetFlags},
	"http":  {flags: httpieFlags, positionalMethod: true},
	"https": {flags: httpieFlags, positionalMethod: true},
	"xh":    {flags: httpieFlags, positionalMethod: true},
	"xhs":   {flags: httpieFlags, positionalMethod: true},
}

// IsHTTPClient reports whether the command is a known HTTP client.
func IsHTTPClient(cmd *Command) bool {
	_, known := httpClientSpecs[cmd.Name]

	return known
}

// ParseHTTPClientCommand parses a curl, wget, httpie or xh invocation into the
// request it sends. The second return is false when the command is not a known
// client or carries no URL.
func ParseHTTPClientCommand(cmd Command) (*HTTPRequest, bool) {
	spec, known := httpClientSpecs[cmd.Name]
	if !known {
		return nil, false
	}

	req := &HTTPRequest{
		Tool:             cmd.Name,
		Body:             cmd.Stdin,
		WorkingDirectory: cmd.WorkingDirectory,
		Location:         cmd.Location,
	}

	state := httpClientState{}

	for i := 0; i < len(cmd.Args); {
		i += req.parseClientArg(cmd.Args, i, &spec, &state)
	}

	if req.URL == "" {
		return nil, false
	}

	req.Method = state.resolveMethod()

	return req, true
}

// httpClientState accumulates the signals needed to resolve the method.
type httpClientState struct {
	explicitMethod   string
	positionalMethod string
	impliedMethod    string
	hasDataItem      bool
}

// resolveMethod applies the precedence every client shares: an explicit method
// flag, then a bare verb, then what a body flag implies, then GET.
func (s *httpClientState) resolveMethod() string {
	switch {
	case s.explicitMethod != "":
		return s.explicitMethod
	case s.positionalMethod != "":
		return s.positionalMethod
	case s.impliedMethod != "":
		return s.impliedMethod
	case s.hasDataItem:
		return methodPOST
	default:
		return methodGET
	}
}

// parseClientArg consumes one argument and returns how many args it used.
func (r *HTTPRequest) parseClientArg(
	args []string,
	idx int,
	spec *httpClientSpec,
	state *httpClientState,
) int {
	arg := args[idx]

	if !strings.HasPrefix(arg, "-") || arg == stdinPath {
		r.parseClientPositional(arg, spec, state)

		return argsPerFlag
	}

	name, value, attached := splitClientFlag(arg, spec)

	flag, known := spec.find(name)
	if !known {
		return argsPerFlag
	}

	used := argsPerFlag

	if !attached && flag.takesValue && idx+1 < len(args) {
		value = args[idx+1]
		used = argsPerFlagWithValue
	}

	r.applyClientFlag(flag, value, state)

	return used
}

// applyClientFlag records what a single flag says about the request.
func (r *HTTPRequest) applyClientFlag(
	flag clientFlag,
	value string,
	state *httpClientState,
) {
	if flag.implies != "" && state.impliedMethod == "" {
		state.impliedMethod = flag.implies
	}

	switch flag.role {
	case roleMethod:
		state.explicitMethod = strings.ToUpper(value)
	case roleURL:
		r.URL = value
	case roleBodyFile:
		r.BodyFile = value
	case roleBody:
		r.collectBody(value)
	case roleNone:
	}
}

// collectBody records inline body text, following curl's "@path" form to a file.
func (r *HTTPRequest) collectBody(value string) {
	if path, isFile := strings.CutPrefix(value, "@"); isFile {
		r.BodyFile = path

		return
	}

	if r.Body != "" {
		r.Body += "\n"
	}

	r.Body += value
}

// parseClientPositional records a bare verb, the URL, or an httpie data item.
func (r *HTTPRequest) parseClientPositional(
	arg string,
	spec *httpClientSpec,
	state *httpClientState,
) {
	if spec.positionalMethod && r.URL == "" && state.positionalMethod == "" &&
		httpMethods[strings.ToUpper(arg)] {
		state.positionalMethod = strings.ToUpper(arg)

		return
	}

	if r.URL == "" {
		r.URL = arg

		return
	}

	// httpie request items ("field=value", "field:=json") make the call a POST.
	if strings.Contains(arg, "=") {
		state.hasDataItem = true

		r.collectBody(arg)
	}
}

// splitClientFlag splits --flag=value and a short flag with an attached value.
func splitClientFlag(arg string, spec *httpClientSpec) (string, string, bool) {
	if name, value, found := strings.Cut(arg, "="); found {
		return name, value, true
	}

	if len(arg) > shortFlagLen && !strings.HasPrefix(arg, "--") {
		short := arg[:shortFlagLen]
		if flag, known := spec.find(short); known && flag.takesValue {
			return short, arg[shortFlagLen:], true
		}
	}

	return arg, "", false
}

// SplitURL separates a URL into its host and its path. The path keeps its
// leading slash so callers can recognise a GitHub Enterprise API prefix. A
// value with no scheme is treated as a path with no host.
func SplitURL(raw string) (string, string) {
	rest, found := strings.CutPrefix(raw, "https://")
	if !found {
		if rest, found = strings.CutPrefix(raw, "http://"); !found {
			return "", raw
		}
	}

	host, path, hasPath := strings.Cut(rest, "/")
	if !hasPath {
		return host, ""
	}

	return host, "/" + path
}
