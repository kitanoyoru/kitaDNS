default: mod fmt test 

mod:
    go mod tidy
    go mod vendor

fmt:
    go mod tidy
    go mod vendor
    go fmt ./...
    golangci-lint run -v --fix ./...

test:
    gotestsum --format standard-verbose -- --covermode atomic --coverpkg ./... --count 1 --race ./...
