BUILD_PATH := ./artifacts
LINUX_BUILD_PATH = $(BUILD_PATH)/linux/gogitver
LINUX_ARM_BUILD_PATH = $(BUILD_PATH)/arm/gogitver
WINDOWS_BUILD_PATH = $(BUILD_PATH)/windows/gogitver.exe
MAC_BUILD_PATH = $(BUILD_PATH)/darwin/gogitver

.PHONY: clean
clean:
	rm -Rf ./artifacts

.PHONY: test
test:
	go test -v ./...

.PHONY: build
build: clean test
	mkdir -p artifacts/linux artifacts/arm artifacts/windows artifacts/darwin
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

package: build
	cd $(BUILD_PATH)/darwin && tar -zcvf ../darwin.tar.gz *
	cd $(BUILD_PATH)/linux && tar -zcvf ../linux.tar.gz *
	cd $(BUILD_PATH)/arm && tar -zcvf ../arm.tar.gz *
	cd $(BUILD_PATH)/windows && zip -r ../windows.zip *
	rm -R $(BUILD_PATH)/darwin $(BUILD_PATH)/linux $(BUILD_PATH)/arm $(BUILD_PATH)/windows
