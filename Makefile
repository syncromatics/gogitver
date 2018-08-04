BUILD_PATH := artifacts/gogitver

version: build
	$(BUILD_PATH)

deps:
	@which dep 2>/dev/null || go get -u ./...
	@dep ensure -v

build: deps
	@go build -o $(BUILD_PATH) cmd/gogitver/main.go

.PHONY: version
