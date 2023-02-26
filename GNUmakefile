BIN_DIR = $(CURDIR)/bin

all: build

build: FORCE
	GOBIN=$(BIN_DIR) go install $(CURDIR)/...

check: vet

vet:
	go vet $(CURDIR)/...

test:
	go test -race -count 1 $(CURDIR)/...

FORCE:

.PHONY: all build check vet test
