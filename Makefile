SHELL := /bin/sh

.PHONY: all frontend hub agent agent-linux stub-frontend test vet fmt-check protocol-check vuln check-public clean

all: frontend hub agent

frontend:
	./scripts/build-frontend.sh

hub: frontend
	mkdir -p bin
	cd hub && GOWORK=off go build -o ../bin/pierops-hub .

agent:
	mkdir -p bin
	cd agent && GOWORK=off go build -o ../bin/pierops-agent .

agent-linux:
	./scripts/package-agent.sh

# Hub packages embed the default theme. Tests and static analysis use a
# placeholder archive when the real frontend has not been built; an existing
# archive is never overwritten.
stub-frontend:
	cd hub && GOWORK=off go run ./internal/stubfrontend

# Full suites. Tests that need public hosts and raw ICMP sockets are skipped
# unless PIEROPS_NETWORK_TESTS=1; command execution tests skip under root.
test: stub-frontend
	cd hub && GOWORK=off go test ./...
	cd agent && GOWORK=off go test ./...

vet: stub-frontend
	cd hub && GOWORK=off go vet ./...
	cd agent && GOWORK=off go vet ./...
	cd agent && GOWORK=off GOOS=windows go vet ./...

# Upstream files are left as imported to keep future syncs reviewable; only
# PierOps-owned packages are held to gofmt here.
PIEROPS_GO_DIRS := hub/internal/access hub/internal/stubfrontend hub/protocol hub/web/agent hub/web/platform agent/internal agent/protocol

fmt-check:
	@unformatted=$$(gofmt -l $(PIEROPS_GO_DIRS)); if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

# The ticket format is security-critical and duplicated on purpose (see
# docs/ARCHITECTURE.md). The two copies must not drift.
protocol-check:
	cmp hub/protocol/v2/ticket.go agent/protocol/v2/ticket.go
	cmp hub/protocol/v2/ticket_test.go agent/protocol/v2/ticket_test.go

vuln: stub-frontend
	cd hub && GOWORK=off go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	cd agent && GOWORK=off go run golang.org/x/vuln/cmd/govulncheck@latest ./...

check-public:
	python3 scripts/check-public.py

clean:
	rm -rf bin frontend/dist hub/web/public/defaultTheme
