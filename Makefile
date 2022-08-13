default: app

app: *.go go.mod go.sum
	go build -o $@ -ldflags="-s -w -linkmode=external" -buildmode=pie -trimpath
	upx $@

.PHONY: docker-build
docker-build:
	docker build .
