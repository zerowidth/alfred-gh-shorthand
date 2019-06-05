package parser

import (
	"regexp"
	"strings"
)

// Parser is a shorthand parser
type Parser struct {
	repoMap     map[string]string
	userMap     map[string]string
	defaultRepo string
	requireRepo bool // require a repository match
	parseRepo   bool // look for a repository match
	parseUser   bool // look for users
	parseIssue  bool // look for issues (#123, 123)
	parsePath   bool // look for /path
	parseQuery  bool // any extra text
}

// Option is a functional option to configure a Parser
type Option func(*Parser)

// NewParser returns a configured Parser
func NewParser(repoMap, userMap map[string]string, defaultRepo string, options ...Option) *Parser {
	parser := &Parser{
		repoMap:     repoMap,
		userMap:     userMap,
		defaultRepo: defaultRepo,
	}
	for _, option := range options {
		option(parser)
	}
	return parser
}

// RequireRepo instructs the parser to require a repository
func RequireRepo(p *Parser) {
	p.parseRepo = true
	p.requireRepo = true
}

// WithRepo instructs the parser to look for a repo match
func WithRepo(p *Parser) { p.parseRepo = true }

// WithUser instructs the parser to look for a user
//
// When this is set alongside WithRepo, a repo will take precedence
func WithUser(p *Parser) { p.parseUser = true }

// WithIssue instructs the parser to look for issue (or project) numbers
func WithIssue(p *Parser) { p.parseIssue = true }

// WithPath instructs the parser to look for a path
func WithPath(p *Parser) { p.parsePath = true }

// WithQuery instructs the parser to match any remaining text as a query
func WithQuery(p *Parser) { p.parseQuery = true }

// Parse parses the given input and returns a result
func (p *Parser) Parse(input string) *NewResult {
	res := &NewResult{}

	if p.parseRepo {
		if repo := userRepoRegexp.FindString(input); len(repo) > 0 {
			// found a repository directly, check for expansion:
			res.SetRepo(repo)
			if shortUser, ok := p.userMap[res.User]; ok {
				res.UserShorthand = res.User
				res.User = shortUser
			}
			input = input[len(repo):]
		} else if user := userRegexp.FindString(input); len(user) > 0 {
			// found a user, see if it's repo shorthand:
			if shortRepo, ok := p.repoMap[user]; ok {
				res.SetRepo(shortRepo)
				res.RepoShorthand = user
				input = input[len(user):]
			} else if p.parseUser {
				// not repo shorthand, but we're allowed to match a user:
				res.User = user
				if shortUser, ok := p.userMap[user]; ok {
					res.UserShorthand = user
					res.User = shortUser
				}
				input = input[len(user):]
			}
		}

		// assign default repository if needed:
		if p.parseRepo && !res.HasRepo() && len(p.defaultRepo) > 0 {
			if p.parseUser && res.HasUser() {
				// if the matched user looks like an issue and there's no further input,
				// use the default repo and use the numeric user as an issue:
				if issue := issueRegexp.FindString(res.User); len(issue) > 0 {

					// if there's still input even after a numeric-looking user, this is
					// invalid. NB this _could_ be valid if parsing a query, but that use
					// case isn't needed/supported
					if len(input) > 0 {
						return &NewResult{}
					}

					res.Issue = res.User
					res.SetRepo(p.defaultRepo)
				}
			} else {
				res.SetRepo(p.defaultRepo)
			}
		}
	}

	// if we don't have a repo assigned by now, there's no match
	if p.requireRepo && !res.HasRepo() {
		return &NewResult{}
	}

	if p.parseIssue {
		if matches := issueRegexp.FindStringSubmatch(input); matches != nil {
			res.Issue = matches[1]
			input = input[len(matches[0]):]
		}
	}

	if p.parsePath {
		if matches := pathRegexp.FindStringSubmatch(input); matches != nil {
			res.Path = matches[1]
			input = input[len(matches[0]):]
		}
	}

	if p.parseQuery {
		// only remove the first leading space, and all trailing spaces
		res.Query = strings.TrimPrefix(strings.TrimRight(input, " "), " ")
	} else if len(input) > 0 {
		res = &NewResult{} // invalid match, there's leftover characters
	}

	return res
}

// Parse takes a user and repo mapping along with an input string and attempts
// to extract a repo, issue, path, or query, using the user and repo mappings
// for shorthand expansion.
//
// bareUser determines whether or not a bare username is allowed as input.
// ignoreNumeric determines whether or not to ignore a bare user if it's
// entirely numeric. if true, numeric-only will be parsed as an issue, not user.
func Parse(repoMap, userMap map[string]string, input string, bareUser, ignoreNumeric bool) Result {
	var res Result

	if r := userRepoRegexp.FindString(input); len(r) > 0 {
		res.SetRepo(r)
		if su, ok := userMap[res.User]; ok {
			res.UserMatch = res.User
			res.User = su
		}
		input = input[len(r):]
	} else if u := userRegexp.FindString(input); len(u) > 0 {
		if sr, ok := repoMap[u]; ok {
			res.SetRepo(sr)
			res.RepoMatch = u
			input = input[len(u):]
		} else if su, ok := userMap[u]; ok {
			res.UserMatch = u
			res.User = su
			input = input[len(u):]
		} else if bareUser && (!ignoreNumeric || !issueRegexp.MatchString(input)) {
			res.User = u
			input = input[len(u):]
		}
	}

	// only remove the first leading space
	res.Query = strings.TrimPrefix(strings.TrimRight(input, " "), " ")

	return res
}

var (
	// using (\A|\z|\W) since \b requires a \w on the left
	userRepoRegexp = regexp.MustCompile(`^([A-Za-z0-9][-A-Za-z0-9]*)/([\w\.\-]*)(\A|\z|\w)`) // user/repo
	userRegexp     = regexp.MustCompile(`^([A-Za-z0-9][-A-Za-z0-9]*)\b`)                     // user
	issueRegexp    = regexp.MustCompile(`^ ?#?([1-9]\d*)$`)
	pathRegexp     = regexp.MustCompile(`^ ?(/\S*)$`)
)
