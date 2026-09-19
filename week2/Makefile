.PHONY: build test clean

build:
	go build -buildmode=exe -o advent-agent ./cmd/advent-agent
	go build -buildmode=exe -o advent-agentd ./cmd/advent-agentd
	./advent-agent --help >/dev/null
	./advent-agentd --help >/dev/null

test:
	go test ./...
	go test -race ./...

clean:
	rm -f advent-agent advent-agentd
