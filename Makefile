all: build
build:
	GOOS=linux CGO_ENABLED=0 go build --ldflags '-extldflags "-static"' -o ick src/main.go

build-docker:
	docker build -t image-capture-kubernetes .
