BIN = bin
BIN_NAME = mqttgateway

.PHONY: all
all: build

$(BIN):
	mkdir $(BIN)

.PHONY: build
build $(BIN)/$(BIN_NAME): $(BIN) vendor
	env CGO_ENABLED=0 go build -o $(BIN)/$(BIN_NAME)

vendor:
	export GO111MODULE=on && go mod init && go mod vendor
