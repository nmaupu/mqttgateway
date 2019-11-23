BIN = bin
BIN_NAME = mqttgateway

.PHONY: all
all: build

$(BIN):
	mkdir $(BIN)

.PHONY: build
build $(BIN)/$(BIN_NAME): $(BIN) vendor
	env CGO_ENABLED=0 go build -o $(BIN)/$(BIN_NAME)

.PHONY: build-x86_64
build-x86_64 $(BIN)/$(BIN_NAME)-x86_64: $(BIN) vendor
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BIN)/$(BIN_NAME)-x86_64

vendor:
	export GO111MODULE=on && go mod init && go mod vendor
