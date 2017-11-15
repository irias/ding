#!/bin/sh
set -e

env GOOS=$GOOS GOARCH=$GOARCH \
NAME=ding \
VERSION=$(git describe --tags | sed 's/^v//') \
GOVERSION=$(go version | cut -f3 -d' ') \
sh -c '
	CGO_ENABLED=0;
	DEST=local/${NAME}-${VERSION:-x}-${GOOS:-x}-${GOARCH:-x}-${GOVERSION:-x}-${BUILDID:-0};
	go build -ldflags "-X main.version=${VERSION:-x}" &&
	mv $NAME $DEST &&
	sh -c "cat assets.zip >>$DEST" &&
	echo release: $NAME $VERSION $GOOS $GOARCH $GOVERSION $DEST
'

env GOOS=$GOOS GOARCH=$GOARCH \
NAME=dingkick \
VERSION=$(git describe --tags | sed 's/^v//') \
GOVERSION=$(go version | cut -f3 -d' ') \
sh -c '
	CGO_ENABLED=0;
	DEST=$PWD/local/${NAME}-${VERSION:-x}-${GOOS:-x}-${GOARCH:-x}-${GOVERSION:-x}-${BUILDID:-0};
	(cd cmd/dingkick && go build -ldflags "-X main.version=${VERSION:-x}") &&
	mv cmd/dingkick/$NAME $DEST &&
	echo release: $NAME $VERSION $GOOS $GOARCH $GOVERSION $DEST
'
