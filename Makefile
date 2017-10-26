run:
	python www-src/build.py
	go build -i
	sherpadoc Ding >assets/ding.json
	./ding local/config.json

build:
	go build -i

frontend:
	python www-src/build.py

test:
	go test -- local/config-test.json

release: clean
	./release.sh

clean:
	go clean
	-rm -r assets
	python www-src/build.py clean
