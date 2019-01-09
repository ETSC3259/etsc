# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: getsc android ios getsc-cross swarm evm all test clean
.PHONY: getsc-linux getsc-linux-386 getsc-linux-amd64 getsc-linux-mips64 getsc-linux-mips64le
.PHONY: getsc-linux-arm getsc-linux-arm-5 getsc-linux-arm-6 getsc-linux-arm-7 getsc-linux-arm64
.PHONY: getsc-darwin getsc-darwin-386 getsc-darwin-amd64
.PHONY: getsc-windows getsc-windows-386 getsc-windows-amd64

GOBIN = $(shell pwd)/build/bin
GO ?= latest

getsc:
	build/env.sh go run build/ci.go install ./cmd/getsc
	@echo "Done building."
	@echo "Run \"$(GOBIN)/getsc\" to launch getsc."

swarm:
	build/env.sh go run build/ci.go install ./cmd/swarm
	@echo "Done building."
	@echo "Run \"$(GOBIN)/swarm\" to launch swarm."

all:
	build/env.sh go run build/ci.go install

android:
	build/env.sh go run build/ci.go aar --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/getsc.aar\" to use the library."

ios:
	build/env.sh go run build/ci.go xcode --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/getsc.framework\" to use the library."

test: all
	build/env.sh go run build/ci.go test

lint: ## Run linters.
	build/env.sh go run build/ci.go lint

clean:
	./build/clean_go_build_cache.sh
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go get -u golang.org/x/tools/cmd/stringer
	env GOBIN= go get -u github.com/kevinburke/go-bindata/go-bindata
	env GOBIN= go get -u github.com/fjl/gencodec
	env GOBIN= go get -u github.com/golang/protobuf/protoc-gen-go
	env GOBIN= go install ./cmd/abigen
	@type "npm" 2> /dev/null || echo 'Please install node.js and npm'
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

swarm-devtools:
	env GOBIN= go install ./cmd/swarm/mimegen

# Cross Compilation Targets (xgo)

getsc-cross: getsc-linux getsc-darwin getsc-windows getsc-android getsc-ios
	@echo "Full cross compilation done:"
	@ls -ld $(GOBIN)/getsc-*

getsc-linux: getsc-linux-386 getsc-linux-amd64 getsc-linux-arm getsc-linux-mips64 getsc-linux-mips64le
	@echo "Linux cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-*

getsc-linux-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/386 -v ./cmd/getsc
	@echo "Linux 386 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep 386

getsc-linux-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/amd64 -v ./cmd/getsc
	@echo "Linux amd64 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep amd64

getsc-linux-arm: getsc-linux-arm-5 getsc-linux-arm-6 getsc-linux-arm-7 getsc-linux-arm64
	@echo "Linux ARM cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep arm

getsc-linux-arm-5:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-5 -v ./cmd/getsc
	@echo "Linux ARMv5 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep arm-5

getsc-linux-arm-6:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-6 -v ./cmd/getsc
	@echo "Linux ARMv6 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep arm-6

getsc-linux-arm-7:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-7 -v ./cmd/getsc
	@echo "Linux ARMv7 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep arm-7

getsc-linux-arm64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm64 -v ./cmd/getsc
	@echo "Linux ARM64 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep arm64

getsc-linux-mips:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips --ldflags '-extldflags "-static"' -v ./cmd/getsc
	@echo "Linux MIPS cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep mips

getsc-linux-mipsle:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mipsle --ldflags '-extldflags "-static"' -v ./cmd/getsc
	@echo "Linux MIPSle cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep mipsle

getsc-linux-mips64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips64 --ldflags '-extldflags "-static"' -v ./cmd/getsc
	@echo "Linux MIPS64 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep mips64

getsc-linux-mips64le:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips64le --ldflags '-extldflags "-static"' -v ./cmd/getsc
	@echo "Linux MIPS64le cross compilation done:"
	@ls -ld $(GOBIN)/getsc-linux-* | grep mips64le

getsc-darwin: getsc-darwin-386 getsc-darwin-amd64
	@echo "Darwin cross compilation done:"
	@ls -ld $(GOBIN)/getsc-darwin-*

getsc-darwin-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=darwin/386 -v ./cmd/getsc
	@echo "Darwin 386 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-darwin-* | grep 386

getsc-darwin-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=darwin/amd64 -v ./cmd/getsc
	@echo "Darwin amd64 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-darwin-* | grep amd64

getsc-windows: getsc-windows-386 getsc-windows-amd64
	@echo "Windows cross compilation done:"
	@ls -ld $(GOBIN)/getsc-windows-*

getsc-windows-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=windows/386 -v ./cmd/getsc
	@echo "Windows 386 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-windows-* | grep 386

getsc-windows-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=windows/amd64 -v ./cmd/getsc
	@echo "Windows amd64 cross compilation done:"
	@ls -ld $(GOBIN)/getsc-windows-* | grep amd64
