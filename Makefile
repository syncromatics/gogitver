BUILD_PATH := artifacts/gogitver
LINUX_BUILD_PATH = $(BUILD_PATH)_linux
LINUX_ARM_BUILD_PATH = $(LINUX_BUILD_PATH)-arm
WINDOWS_BUILD_PATH = $(BUILD_PATH)_windows.exe

version: build
	$(LINUX_BUILD_PATH)

deps:
	@which dep 2>/dev/null || go get -u ./...
	@dep ensure -v

build:
	GOOS=linux GOARCH=amd64 go build -o $(LINUX_BUILD_PATH) cmd/gogitver/main.go
	GOOS=linux GOARCH=arm go build -o $(LINUX_ARM_BUILD_PATH) cmd/gogitver/main.go
	GOOS=windows GOARCH=amd64 go build -o $(WINDOWS_BUILD_PATH) cmd/gogitver/main.go

.PHONY: version
