#!/bin/bash
set -e

DIR=$(dirname $0)

echo "Compiling..."
go get github.com/levavakian/rowm
go get github.com/levavakian/rowm/cmd/rowmbright
echo "Installing to global directories"
rm /usr/share/xsessions/rowm.desktop | true
mkdir -p /usr/local/share/wingo/
mkdir -p /usr/share/xsessions
cp $DIR/resources/dejavu/DejaVuSans.ttf /usr/local/share/wingo/DejaVuSans.ttf
cp $DIR/resources/nofont/write-your-password-with-this-font.ttf  /usr/local/share/wingo/write-your-password-with-this-font.ttf
cp $DIR/resources/rowm.desktop /usr/share/xsessions/rowm.desktop
GOLOC=${GOPATH:-~/go}
chmod u+s $GOLOC/bin/rowmbright
