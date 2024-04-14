BINARY_NAME=dora-exporter
dist:
	CGO_ENABLED=0 go build -o $(BINARY_NAME) cmd/main.go
deps:
	go mod download
test-ci:
	go test -v ./...
test-unit:
	go test -v ./...
test-functional:
	if [ -z "${GITHUB_TOKEN}" ] ; then echo "Requires GITHUB_TOKEN environmnent variable" ; false ; fi
	go test -v ./... -tags=integration
run:
	go run cmd/main.go
