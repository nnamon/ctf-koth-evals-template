.PHONY: all clean test

all: engine/engine

engine/engine: engine/main.go engine/go.mod
	cd engine && go build -o engine .

clean:
	rm -f engine/engine

test: all
	rm -rf /tmp/regex-count-test
	cp -r example /tmp/regex-count-test
	./wrapper --workdir=/tmp/regex-count-test
	cat /tmp/regex-count-test/result.json
