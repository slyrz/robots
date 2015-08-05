package robots_test

import (
	"bufio"
	"bytes"
	"github.com/slyrz/robots"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"
)

type Test struct {
	UserAgent string
	Path      string
	Allow     bool
}

var lineRegex = regexp.MustCompile(`(?i)^#\s*(allow|disallow)\s*,\s*(.*)\s*,\s*(.*)\s*$`)

func parseTest(text string) *Test {
	fields := lineRegex.FindStringSubmatch(text)
	if len(fields) != 4 {
		return nil
	}
	return &Test{
		Allow:     strings.ToLower(fields[1]) == "allow",
		UserAgent: fields[2],
		Path:      fields[3],
	}
}

func loadTestFile(path string) ([]*Test, []byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	tests := make([]*Test, 0)
	reader := bufio.NewReader(file)
	for {
		start, err := reader.Peek(1)
		if err != nil {
			return nil, nil, err
		}
		if string(start) != "#" {
			break
		}
		line, err := reader.ReadString(byte('\n'))
		if err != nil {
			return nil, nil, err
		}
		if test := parseTest(line); test != nil {
			tests = append(tests, test)
		}
	}
	data, err := ioutil.ReadAll(reader)
	return tests, data, err
}

func runTest(t *testing.T, path string) {
	t.Logf("running test %s", path)

	tests, data, err := loadTestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	userAgents := make(map[string]*robots.Robots)
	for _, test := range tests {
		if _, ok := userAgents[test.UserAgent]; !ok {
			userAgents[test.UserAgent] = robots.New(bytes.NewReader(data), test.UserAgent)
		}
	}
	for _, test := range tests {
		t.Logf("user-agent=%q, path=%q, allow=%v", test.UserAgent, test.Path, test.Allow)
		if userAgents[test.UserAgent].Allow(test.Path) != test.Allow {
			t.Errorf("expected %v, got %v", test.Allow, !test.Allow)
		}
	}
}

func TestAll(t *testing.T) {
	dir, err := os.Open("testdata")
	if err != nil {
		t.Fatal(err)
	}
	names, err := dir.Readdirnames(-1)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		runTest(t, "testdata/"+name)
	}
}

func Benchmark(b *testing.B) {
	tests, data, err := loadTestFile("testdata/benchmark.txt")
	if err != nil {
		b.Fatal(err)
	}
	robots := robots.New(bytes.NewReader(data), "foo")

	b.ResetTimer()
	j := 0
	for i := 0; i < b.N; i++ {
		if robots.Allow(tests[j].Path) != tests[j].Allow {
			b.Fatal("benchmark failed")
		}
		j = (j + 1) % len(tests)
	}
}
