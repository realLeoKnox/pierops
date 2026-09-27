SHELL := /bin/sh

.PHONY: all frontend hub agent test check-public clean

all: frontend hub agent

frontend:
	./scripts/build-frontend.sh

hub: frontend
	mkdir -p bin
	cd hub && GOWORK=off go build -o ../bin/pierops-hub .

agent:
	mkdir -p bin
	cd agent && GOWORK=off go build -o ../bin/pierops-agent .

test:
	cd hub && GOWORK=off go test ./protocol/v2/... ./web/agent/... ./web/filemanager/...
	cd agent && GOWORK=off go test ./server ./terminal -run 'Test(File|Create|Copy|List|Resolve|Legacy|Delete|Send|Receive|Upload|First|Commit|Cancel|RunSwitch|RunTask|BuildTask|Append|Motd)'

check-public:
	python3 scripts/check-public.py

clean:
	rm -rf bin frontend/dist hub/web/public/defaultTheme
