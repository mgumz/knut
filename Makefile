VERSION=1.5.0
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_HASH=$(shell git rev-parse HEAD)

RELEASES=bin/knut-$(VERSION).linux.amd64 \
		 bin/knut-$(VERSION).linux.arm64 \
		 bin/knut-$(VERSION).linux.mips64 \
		 bin/knut-$(VERSION).windows.amd64.exe \
		 bin/knut-$(VERSION).freebsd.amd64 \
		 bin/knut-$(VERSION).darwin.amd64


LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.GitHash=$(GIT_HASH)"


releases: $(RELEASES)

bin/knut-$(VERSION).linux.mips64: bin
	env GOOS=linux GOARCH=mips64 CGO_ENABLED=0 go build $(LDFLAGS) -o $@

bin/knut-$(VERSION).linux.amd64: bin
	env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $@

bin/knut-$(VERSION).linux.arm64: bin
	env GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $@

bin/knut-$(VERSION).windows.amd64.exe: bin
	env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $@

bin/knut-$(VERSION).darwin.amd64: bin
	env GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $@

bin/knut-$(VERSION).freebsd.amd64: bin
	env GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $@

bin:
	mkdir $@
