APP_NAME=grab

vet:
	go vet ./...

build:
	go build -o ./grab ./cmd/grab

lint:
	golangci-lint run ./...

test:
	go test -v ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html