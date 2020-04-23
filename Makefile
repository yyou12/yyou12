all: build
.PHONY: all

OUT_DIR=bin
build:
	mkdir -p "${OUT_DIR}"
	go build -o "${OUT_DIR}" "cmd/extended-platform-tests/extended-platform-tests.go"

clean:
	$(RM) ./cmd/extended-platform-tests/extended-platform-tests
.PHONY: clean
