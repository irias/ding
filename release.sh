#!/bin/sh
mkdir local assets 2>/dev/null

sherpadoc Ding >assets/ding.json &&
env GOOS=linux GOARCH=amd64 \
NAME=$(basename $PWD) \
VERSION=$(git describe --tags | sed 's/^v//') \
GOVERSION=$(go version | cut -f3 -d' ') \
sh -c '
	CGO_ENABLED=0;
	DEST=local/${NAME}-${VERSION:-x}-${GOOS:-x}-${GOARCH:-x}-${GOVERSION:-x}-${BUILDID:-0};
	go build -ldflags "-X main.version=${VERSION:-x}" &&
	(rm assets.zip 2>/dev/null; cd assets && zip -qr0 ../assets.zip .) &&
	mv ${NAME} $DEST &&
	sh -c "cat assets.zip >>$DEST" &&
	echo release: ding $VERSION $GOOS $GOARCH $GOVERSION $DEST
' &&
exec env GOOS=linux GOARCH=amd64 \
NAME=dingkick \
VERSION=$(git describe --tags | sed 's/^v//') \
GOVERSION=$(go version | cut -f3 -d' ') \
sh -c '
	CGO_ENABLED=0;
	DEST=$PWD/local/${NAME}-${VERSION:-x}-${GOOS:-x}-${GOARCH:-x}-${GOVERSION:-x}-${BUILDID:-0};
	(cd cmd/dingkick && go build -ldflags "-X main.version=${VERSION:-x}") &&
	mv cmd/dingkick/${NAME} $DEST &&
	echo release: ${NAME} $VERSION $GOOS $GOARCH $GOVERSION $DEST
'
