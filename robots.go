// Package robots implements a robots.txt parser for the robots exclusion
// protocol.
package robots

import (
	"bufio"
	"io"
	"strconv"
	"strings"
	"time"
)

func splitRecord(line string) (string, string, bool) {
	pos := strings.IndexRune(line, ':')
	if pos < 0 {
		return "", "", true
	}
	return cleanField(line[:pos]), cleanValue(line[pos+1:]), false
}

func cleanField(text string) string {
	return strings.TrimSpace(strings.ToLower(text))
}

func cleanValue(text string) string {
	if pos := strings.IndexRune(text, '#'); pos >= 0 {
		text = text[:pos]
	}
	return strings.TrimSpace(text)
}

// RuleType defines the type of a rule.
type RuleType uint

// TypeAllow means the rule allows a path to be crawled.
// TypeDisallow means the rule disallows crawling.
const (
	TypeDisallow RuleType = iota
	TypeAllow
)

// Rule stores a parsed allow/disallow record found in the robots.txt file.
type Rule struct {
	Type    RuleType
	Length  int
	Equals  string   // Matches the whole path.
	Prefix  string   // Matches the start of a path.
	Suffix  string   // Matches the end of a path.
	Needles []string // Matches anything inside a path.
}

// splitPathValue splits a path value into its components separated by
// wildcards, e.g. it turns "/foo*bar/" into []string{"/foo", "bar/"}.
// The return value hasPrefix is true if the path value matches a path
// prefix. The return value hasSuffix is true if the path value matches
// a path suffix.
func splitPathValue(value string) (parts []string, hasPrefix bool, hasSuffix bool) {
	parts = strings.Split(value, "*")
	// If the first item is "", the rule starts with a wildcard "*", which
	// means it has no prefix.
	hasPrefix = parts[0] != ""
	// If the last item ends with "$", the rule matches a suffix.
	hasSuffix = strings.HasSuffix(parts[len(parts)-1], "$")
	// If the prefix does not start with a "/", add one manually.
	// This conforms to Google's interpretation.
	if hasPrefix {
		if parts[0][0] != '/' {
			parts[0] = "/" + parts[0]
		}
	}
	// If there's a suffix, remove the trailing "$".
	if hasSuffix {
		parts[len(parts)-1] = strings.TrimSuffix(parts[len(parts)-1], "$")
	}
	return
}

// cutOff removes the first and last item of array.
func cutOff(array []string, first, last bool) []string {
	if len(array) > 0 && first {
		array = array[1:]
	}
	if len(array) > 0 && last {
		array = array[:len(array)-1]
	}
	return array
}

// NewRule creates a new rule from the value element of an allow/disallow
// record found in the robots.txt file.
func NewRule(ruleType RuleType, value string) *Rule {
	rule := &Rule{
		Type:   ruleType,
		Length: len(value),
	}
	parts, hasPrefix, hasSuffix := splitPathValue(value)
	if hasPrefix {
		rule.Prefix = parts[0]
	}
	if hasSuffix {
		rule.Suffix = parts[len(parts)-1]
	}
	// If this is the case, the rule was something like "/foo$" and matches
	// the whole path.
	if hasPrefix && hasSuffix && len(parts) == 1 {
		rule.Equals, rule.Prefix, rule.Suffix = parts[0], "", ""
	}
	// After prefix and suffix were removed from the parts array, the
	// remaining items are the needles that have to be found anywhere
	// inside the string.
	needles := cutOff(parts, hasPrefix, hasSuffix)
	for _, needle := range needles {
		if needle != "" {
			rule.Needles = append(rule.Needles, needle)
		}
	}
	return rule
}

// Match returns true if the rule matches the given path value.
func (r *Rule) Match(path string) bool {
	if r.Equals != "" {
		return r.Equals == path
	}
	if r.Prefix != "" && !strings.HasPrefix(path, r.Prefix) {
		return false
	}
	if r.Suffix != "" && !strings.HasSuffix(path, r.Suffix) {
		return false
	}
	if len(r.Needles) > 0 {
		// Remove prefix and suffix, otherwise the needles might accidentally
		// match them.
		path = path[len(r.Prefix) : len(path)-len(r.Suffix)]
		// Find the needles. Remember that needles were separated by a *,
		// so the needle at position i must precede needle at position i + 1
		// in path.
		for i := 0; i < len(r.Needles); i++ {
			p := strings.Index(path, r.Needles[i])
			if p < 0 {
				return false
			}
			path = path[p+len(r.Needles[i]):]
		}
	}
	return true
}

// Group stores a group found in a robots.txt file. This type is only used
// during parsing.
type Group struct {
	UserAgents []string
	Allow      []string
	Disallow   []string
	CrawlDelay string
}

// HasMembers returns true if the group has one or more of the following
// members: allow, disallow and crawl-delay.
func (g *Group) HasMembers() bool {
	return len(g.Allow) != 0 || len(g.Disallow) != 0 || g.CrawlDelay != ""
}

// HasUserAgents returns true if the group has one or more user-agents.
func (g *Group) HasUserAgents() bool {
	return len(g.UserAgents) != 0
}

// Matches returns true if one of the group's user-agents matches the
// given name. The second return value is the length of the match. The
// length is zero for no match and wildcard matches.
func (g *Group) Matches(name string) (bool, int) {
	match, length := false, 0
	for _, agent := range g.UserAgents {
		// A wildcard matches all agents, but it's also the weakest match
		// according to Google, so keep the length set to zero.
		if agent == "*" {
			match = true
		}
		if strings.HasPrefix(name, agent) {
			match = true
			if length < len(agent) {
				length = len(agent)
			}
		}
	}
	return match, length
}

// Groups contain all groups found in a robots.txt file. This type is only used
// during parsing.
type Groups []*Group

// NewGroups parses a robots.txt file and returns all groups found in it.
func NewGroups(r io.Reader) (groups Groups) {
	active := &Group{}
	// Ignore possible UTF-8 byte order mark at the beginning of the
	// robots.txt file, just like Google does.
	buffer := bufio.NewReader(r)
	if data, err := buffer.Peek(3); err == nil && string(data) == "\xef\xbb\xbf" {
		buffer.Read(data)
	}
	scanner := bufio.NewScanner(buffer)
	for scanner.Scan() {
		field, value, ignore := splitRecord(scanner.Text())
		if ignore {
			continue
		}
		switch field {
		case "allow":
			active.Allow = append(active.Allow, value)
		case "disallow":
			active.Disallow = append(active.Disallow, value)
		case "crawl-delay", "crawldelay":
			active.CrawlDelay = value
		case "user-agent", "useragent":
			// If the active group has group members, the current user-agent
			// record marks the start of a new group.
			if active.HasMembers() {
				// Only add this group if it was started with one or more
				// user-agent records.
				if active.HasUserAgents() {
					groups = append(groups, active)
				}
				active = &Group{}
			}
			// Wildcards at the end of a user-agent are superfluous since
			// we perform prefix matches.
			// The user-agent is case-insensitive.
			if value != "*" {
				value = strings.TrimRight(value, "*")
				value = strings.ToLower(value)
			}
			active.UserAgents = append(active.UserAgents, value)
		}
	}
	if active.HasMembers() && active.HasUserAgents() {
		groups = append(groups, active)
	}
	return groups
}

// Find returns the group that belongs to user-agent name. If no matching group
// is found, nil is returned.
func (g Groups) Find(name string) (result *Group) {
	// User-agent names are case-insensitive.
	name = strings.ToLower(name)
	// Google says:
	// 	The crawler must determine the correct group of records by finding the
	// 	group with the most specific user-agent that still matches.
	longest := -1
	for _, group := range g {
		if match, length := group.Matches(name); match {
			if length > longest {
				result, longest = group, length
			}
		}
	}
	return
}

// Robots stores all relevant rules for a predefined user-agent.
type Robots struct {
	CrawlDelay time.Duration
	Rules      []*Rule
}

// add adds a rule to the struct.
func (r *Robots) add(rule *Rule) {
	if rule.Length > 0 {
		r.Rules = append(r.Rules, rule)
	}
}

// New parses the robots.txt file in r and returns all Rules relevant
// for the given user-agent.
func New(r io.Reader, useragent string) *Robots {
	group := NewGroups(r).Find(useragent)
	if group == nil {
		return &Robots{}
	}
	result := &Robots{}
	for _, value := range group.Allow {
		result.add(NewRule(TypeAllow, value))
	}
	for _, value := range group.Disallow {
		result.add(NewRule(TypeDisallow, value))
	}
	if group.CrawlDelay != "" {
		if secs, err := strconv.Atoi(group.CrawlDelay); err == nil {
			result.CrawlDelay = time.Duration(secs) * time.Second
		}
	}
	return result
}

// Allow returns true if the parsed robots.txt file allows the given path to
// be crawled.
func (r *Robots) Allow(path string) bool {
	var match *Rule
	for _, rule := range r.Rules {
		if rule.Match(path) {
			if match == nil || rule.Length > match.Length {
				match = rule
			}
		}
	}
	return match == nil || match.Type == TypeAllow
}
