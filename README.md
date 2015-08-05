# robots

This package provides a robots.txt parser for the Robots Exclusion
Protocol in the Go programming language.

The implementation follows [Google's Robots.txt Specification](https://developers.google.com/webmasters/control-crawl-index/docs/robots_txt).

The code is simple and straightforward. The structs exposed by this package consist
of basic data types only, making it easy to encode and decode them using one of Go's
`encoding` packages. And though performance wasn't a design goal, this package should
never become your program's bottleneck.

### Installation

Run

```shell
go get github.com/slyrz/robots
```

### Example

```golang
robots := robots.New(file, "your-user-agent")
if robots.Allow("/some/path") {
	// Crawl it!
	// ...
}
```

### License

robots is released under MIT license.
You can find a copy of the MIT License in the [LICENSE](./LICENSE) file.
