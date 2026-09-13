.PHONY: build test clean

build:
	go build -buildmode=exe -o deepseek-agent ./cmd/deepseek-agent
	go build -buildmode=exe -o deepseek-agentd ./cmd/deepseek-agentd
	./deepseek-agent --help >/dev/null
	./deepseek-agentd --help >/dev/null

test:
	go test ./...
	go test -race ./...

clean:
	rm -f deepseek-agent deepseek-agentd
