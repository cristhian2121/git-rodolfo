GO       ?= go
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X github.com/lean-tech/git-rodolfo/internal/cli.Version=$(VERSION)
DIST     := dist

# git-rodolfo needs no C bindings, so cgo is off everywhere: it's what
# keeps builds fully static and portable, and it sidesteps a real failure
# mode this project hit — with cgo on, `update`'s use of net/http makes
# the binary link against libSystem's resolver, and on at least one dev
# machine (mismatched Xcode Command Line Tools vs. the actual macOS SDK)
# that produced a binary dyld refused to even start
# ("missing LC_UUID load command"). CGO_ENABLED=0 avoids that class of
# toolchain fragility entirely, not just paper over one broken machine.
export CGO_ENABLED = 0

.PHONY: build test lint install clean release

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o git-rodolfo ./cmd/git-rodolfo

test:
	$(GO) vet ./...
	@GOOS="$$($(GO) env GOOS)"; \
	if [ "$$GOOS" = "linux" ] || [ "$$GOOS" = "windows" ]; then \
		CGO_ENABLED=1 $(GO) test ./... -race; \
	else \
		$(GO) test ./... -race; \
	fi

# Installs into $GOPATH/bin (usually ~/go/bin) as `git-rodolfo`. Make sure
# that directory is on PATH so `git rodolfo <command>` finds it too
# (PRD §12.1 — Git looks up `git-rodolfo` via PATH).
install:
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/git-rodolfo

clean:
	rm -rf $(DIST) git-rodolfo

# Cross-compiles the release matrix (PRD §19.1/RNF-02: macOS + Linux +
# Windows). Each binary is named git-rodolfo-<version>-<os>-<arch>; Unix
# platforms ship as tar.gz, Windows as zip (PRD 2 RF-49, Windows convention
# — also avoids tar's execute-bit handling, meaningless for a .exe).
release: clean
	mkdir -p $(DIST)
	$(foreach GOOS,darwin linux, \
		$(foreach GOARCH,amd64 arm64, \
			GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -ldflags "$(LDFLAGS)" \
				-o $(DIST)/git-rodolfo-$(VERSION)-$(GOOS)-$(GOARCH)/git-rodolfo \
				./cmd/git-rodolfo && \
			tar -C $(DIST)/git-rodolfo-$(VERSION)-$(GOOS)-$(GOARCH) -czf \
				$(DIST)/git-rodolfo-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz git-rodolfo ; \
		) \
	)
	$(foreach GOARCH,amd64 arm64, \
		GOOS=windows GOARCH=$(GOARCH) $(GO) build -ldflags "$(LDFLAGS)" \
			-o $(DIST)/git-rodolfo-$(VERSION)-windows-$(GOARCH)/git-rodolfo.exe \
			./cmd/git-rodolfo && \
		(cd $(DIST)/git-rodolfo-$(VERSION)-windows-$(GOARCH) && zip -q ../git-rodolfo-$(VERSION)-windows-$(GOARCH).zip git-rodolfo.exe) ; \
	)
	cd $(DIST) && shasum -a 256 *.tar.gz *.zip > checksums.txt
