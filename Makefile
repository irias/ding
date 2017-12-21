run: build
	./ding serve local/config.json

run-root: build
	sudo ./ding serve local/config.json

build:
	python www-src/build.py
	go build -i
	sherpadoc Ding >assets/ding.json

frontend:
	python www-src/build.py

test:
	go vet
	golint
	go test -cover -- local/config-test.json

release:
	-rm assets.zip 2>/dev/null
	(cd assets && zip -qr0 ../assets.zip .)
	env GOOS=linux GOARCH=amd64 ./release.sh
	env GOOS=linux GOARCH=arm GOARM=6 ./release.sh
	env GOOS=linux GOARCH=arm64 ./release.sh
	env GOOS=darwin GOARCH=amd64 ./release.sh
	env GOOS=openbsd GOARCH=amd64 ./release.sh

clean:
	go clean
	-rm -r assets
	python www-src/build.py clean

setup:
	-mkdir -p node_modules/.bin
	npm install jshint@2.9.3 node-sass@4.7.2
