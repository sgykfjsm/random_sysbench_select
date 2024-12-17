build:
	@rm -f random_select

	GOWORK=off go mod tidy
	GOWORK=off go mod vendor
	GOWORK=off go build -ldflags "-w -s" -o random_select main.go

build_linux:
	@rm -f random_select_linux

	GOWORK=off go mod tidy
	GOWORK=off go mod vendor
	GOWORK=off GOOS=linux GOARCH=amd64 go build -ldflags "-w -s" -o random_select_linux main.go

all: build build_linux
