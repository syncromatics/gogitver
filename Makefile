BUILD_PATH := artifacts/gogitver
LINUX_BUILD_PATH = $(BUILD_PATH)_linux
LINUX_ARM_BUILD_PATH = $(LINUX_BUILD_PATH)-arm
WINDOWS_BUILD_PATH = $(BUILD_PATH)_windows.exe
MAC_BUILD_PATH = $(BUILD_PATH)_darwin
export VERSION=$(shell gogitver)

.PHONY: version
version: build
	$(LINUX_BUILD_PATH)

.PHONY: clean
clean:
	rm -Rf ./artifacts

.PHONY: test
test:
	go test -v ./...

.PHONY: build
build: clean test
	GOOS=linux GOARCH=amd64 go build -o $(LINUX_BUILD_PATH) cmd/gogitver/main.go
	GOOS=linux GOARCH=arm go build -o $(LINUX_ARM_BUILD_PATH) cmd/gogitver/main.go
	GOOS=darwin GOARCH=amd64 go build -o $(MAC_BUILD_PATH) cmd/gogitver/main.go
	GOOS=windows GOARCH=amd64 go build -o $(WINDOWS_BUILD_PATH) cmd/gogitver/main.go

.PHONY: build-debian-package
build-debian-package: build
	mkdir -p artifacts/debian/DEBIAN
	mkdir -p artifacts/debian/usr/local/bin/
	cat ./build/debian/control.template | envsubst > ./artifacts/debian/DEBIAN/control
	cp $(LINUX_BUILD_PATH) artifacts/debian/usr/local/bin/gogitver
	dpkg-deb --build ./artifacts/debian ./artifacts/gogitver_amd64.deb
	rm -R ./artifacts/debian

.PHONY: build-snap
build-snap: build
	mkdir -p artifacts/snap/snap
	mkdir -p artifacts/snap/source
	cp $(LINUX_BUILD_PATH) artifacts/snap/source/gogitver
	cat ./build/snap/snapcraft.yaml | envsubst > ./artifacts/snap/snap/snapcraft.yaml
	cd ./artifacts/snap && snapcraft clean gogitver -s pull
	cd ./artifacts/snap && snapcraft
	mv ./artifacts/snap/gogitver*.snap ./artifacts
	rm -R ./artifacts/snap

package: build-debian-package
