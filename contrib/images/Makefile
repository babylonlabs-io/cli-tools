DOCKER = $(shell which docker)

.PHONY: cli-tools cli-tools-rmi

cli-tools:
	$(DOCKER) build --tag babylonchain/cli-tools -f cli-tools/Dockerfile \
				$(shell git rev-parse --show-toplevel)

cli-tools-rmi:
	$(DOCKER) rmi babylonchain/cli-tools 2>/dev/null; true
