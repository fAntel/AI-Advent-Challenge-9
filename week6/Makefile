.PHONY: build test
build:
	go build -buildmode=exe -o advent-agent ./cmd/advent-agent
	go build -buildmode=exe -o advent-agentd ./cmd/advent-agentd
	go build -buildmode=exe -o git-mcp ./cmd/git-mcp
	go build -buildmode=exe -o support-bot ./cmd/support-bot
	go build -buildmode=exe -o mattermost-mcp ./cmd/mattermost-mcp
	./advent-agent --help >/dev/null
	./advent-agentd --help >/dev/null
	./git-mcp --help >/dev/null
	./support-bot --help >/dev/null
	./mattermost-mcp --help >/dev/null
	python3 scripts/repo_index.py --help >/dev/null

test:
	python3 -m unittest discover -s scripts -p 'test_*.py'
	PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s skills/review-bitbucket-pr/scripts -p 'test_*.py'
	go test ./...
	go test -race ./...
