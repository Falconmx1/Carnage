BINARY  := carnage
VERSION ?= dev
LDFLAGS := -s -w -X main.Version=$(VERSION)
PKG     := ./cmd/carnage
OUTDIR  := dist

.PHONY: all build vet test tidy clean linux windows darwin arm mips

all: build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) $(PKG)

vet:
	go vet ./...

test:
	go test ./...

tidy:
	go mod tidy

linux:
	mkdir -p $(OUTDIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-linux-amd64 $(PKG)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-linux-arm64 $(PKG)

arm:
	mkdir -p $(OUTDIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-linux-armv7 $(PKG)

mips:
	mkdir -p $(OUTDIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=mips go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-linux-mips $(PKG)
	CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-linux-mipsle $(PKG)

windows:
	mkdir -p $(OUTDIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-windows-amd64.exe $(PKG)

darwin:
	mkdir -p $(OUTDIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-darwin-amd64 $(PKG)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-darwin-arm64 $(PKG)

clean:
	rm -rf $(OUTDIR) $(BINARY) $(BINARY).exe
