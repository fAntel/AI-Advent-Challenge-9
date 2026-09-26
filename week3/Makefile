.PHONY: build test clean

build:
	go build -buildmode=exe -o advent-agent ./cmd/advent-agent
	go build -buildmode=exe -o advent-agentd ./cmd/advent-agentd
	go build -buildmode=exe -o homebox-mcp ./cmd/homebox-mcp
	./advent-agent --help >/dev/null
	./advent-agentd --help >/dev/null
	./homebox-mcp --help >/dev/null

test:
	go test ./...
	go test -race ./...

clean:
	rm -f advent-agent advent-agentd homebox-mcp
