BIN := ./bin/sam
PREFIX ?= $(HOME)/.local
INSTALL_DIR := $(PREFIX)/bin

.PHONY: build install clean

build:
	go build -o $(BIN) ./cmd/sam

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BIN) $(INSTALL_DIR)/sam

clean:
	rm -rf ./bin
