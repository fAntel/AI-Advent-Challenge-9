.PHONY: build test

build:
	go build -o local-llm .
	./local-llm --help >/dev/null

test:
	go test ./...
