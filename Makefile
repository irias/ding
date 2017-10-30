run: build0
	./ding local/config.json

build0:
	python www-src/build.py
	go build -i
	sherpadoc Ding >assets/ding.json

frontend:
	python www-src/build.py

test:
	go test -cover -- local/config-test.json

release: clean
	./release.sh

clean:
	go clean
	-rm -r assets
	python www-src/build.py clean
