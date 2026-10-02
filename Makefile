SHELL := /bin/sh

.PHONY: all frontend hub agent agent-linux test check-public clean

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

test:
	cd hub && GOWORK=off go test ./internal/access ./web/router -run 'TestPierOps|TestNodeAction|TestAPIKeyRotation|TestPolicyValidation|TestRPCScope|TestMissingAudit|TestLegacyOwner'
	cd hub && GOWORK=off go test ./protocol/v2/... ./web/agent/... ./web/filemanager/...
	cd agent && GOWORK=off go test ./protocol/v2 ./internal/localpolicy ./server ./terminal -run 'TestTicket|TestLocal|Test(File|Create|Copy|List|Resolve|Legacy|Delete|Send|Receive|Upload|First|Commit|Cancel|RunSwitch|RunTask|BuildTask|Append|Motd)'

check-public:
	python3 scripts/check-public.py

clean:
	rm -rf bin frontend/dist hub/web/public/defaultTheme
