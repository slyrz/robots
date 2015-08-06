package robots_test

import (
	"bufio"
	"bytes"
	"fmt"
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

func (t *Test) String() string {
	return fmt.Sprintf("user-agent=%q, path=%q, allow=%v", t.UserAgent, t.Path, t.Allow)
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
	tests, data, err := loadTestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("running file %s: %d tests", path, len(tests))

	userAgents := make(map[string]*robots.Robots)
	for _, test := range tests {
		if _, ok := userAgents[test.UserAgent]; !ok {
			userAgents[test.UserAgent] = robots.New(bytes.NewReader(data), test.UserAgent)
		}
	}
	for _, test := range tests {
		if userAgents[test.UserAgent].Allow(test.Path) != test.Allow {
			t.Errorf("test %v failed", test)
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
