run: build
	./ding serve local/config.json

run-root: build
	sudo ./ding serve local/config.json

fabricate/fabricate: fabricate/fabricate.go fabricate/fablib.go
	go build -o $@ ./fabricate

vendor/bitbucket.org/mjl/sherpa/cmd/sherpadoc/sherpadoc: vendor/bitbucket.org/mjl/sherpa/cmd/sherpadoc/*.go
	go build -o $@ ./vendor/bitbucket.org/mjl/sherpa/cmd/sherpadoc

build: fabricate/fabricate vendor/bitbucket.org/mjl/sherpa/cmd/sherpadoc/sherpadoc
	go build -i
	./fabricate/fabricate install
	vendor/bitbucket.org/mjl/sherpa/cmd/sherpadoc/sherpadoc Ding >assets/ding.json

frontend: fabricate/fabricate
	./fabricate/fabricate install

test:
	go vet
	golint
	go test -cover -- local/config-test.json

coverage:
	go test -coverprofile=coverage.out -test.outputdir . -- local/config-test.json
	go tool cover -html=coverage.out

fmt:
	gofmt -w *.go

release:
	(cd assets && zip -qr0 ../assets.zip .)
	env GOOS=linux GOARCH=amd64 ./release.sh
	env GOOS=linux GOARCH=arm GOARM=6 ./release.sh
	env GOOS=linux GOARCH=arm64 ./release.sh
	env GOOS=darwin GOARCH=amd64 ./release.sh
	env GOOS=openbsd GOARCH=amd64 ./release.sh

clean: fabricate/fabricate
	go clean
	-rm -r assets assets.zip
	./fabricate/fabricate clean
	go clean ./fabricate
	go clean ./vendor/bitbucket.org/mjl/sherpa/cmd/sherpadoc

setup:
	-mkdir -p node_modules/.bin
	npm install jshint@2.9.3 node-sass@4.7.2
