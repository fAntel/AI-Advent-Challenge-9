.PHONY: build test clean

build:
	.venv/bin/python -m pip install --no-build-isolation .
	.venv/bin/kotlin-index --help >/dev/null
	go build -buildmode=exe -o advent-agent ./cmd/advent-agent
	go build -buildmode=exe -o advent-agentd ./cmd/advent-agentd
	go build -buildmode=exe -o homebox-mcp ./cmd/homebox-mcp
	go build -buildmode=exe -o budget-mcp ./cmd/budget-mcp
	go build -buildmode=exe -o report-mcp ./cmd/report-mcp
	go build -buildmode=exe -o supplier-mcp ./cmd/supplier-mcp
	./advent-agent --help >/dev/null
	./advent-agentd --help >/dev/null
	./homebox-mcp --help >/dev/null
	./budget-mcp --help >/dev/null
	./report-mcp --help >/dev/null
	./supplier-mcp --help >/dev/null

test:
	.venv/bin/python -m unittest discover -s tests -v
	go test ./...
	go test -race ./...

clean:
	rm -f advent-agent advent-agentd homebox-mcp budget-mcp report-mcp supplier-mcp
