.PHONY: build test vet clean run

BINARY=taskpilot
CMD_DIR=cmd/taskpilot

build:
	CGO_ENABLED=0 go build -trimpath -o $(BINARY) ./$(CMD_DIR)

test:
	go test ./... -count=1 -race

vet:
	go vet ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) taskpilot.json

docker-build:
	docker build -t taskpilot .
