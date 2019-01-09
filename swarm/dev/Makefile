.PHONY: build cluster test

default: build

build:
	go build -o bin/swarm    github.com/ETSC3259/etsc/cmd/swarm
	go build -o bin/getsc     github.com/ETSC3259/etsc/cmd/getsc
	go build -o bin/bootnode github.com/ETSC3259/etsc/cmd/bootnode

cluster: build
	scripts/boot-cluster.sh

test:
	go test -v github.com/ETSC3259/etsc/swarm/...
