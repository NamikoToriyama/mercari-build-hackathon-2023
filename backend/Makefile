.PHONY: setup
setup:
	go install github.com/golang/mock/mockgen@v1.6.0

.PHONY: gen
gen:
	go generate ./...

.PHONY: test
test:
	go test -shuffle=on -race ./...
