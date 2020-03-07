#!/usr/bin/env bash

if [ -z "$1" ]; then
	echo "usage: $0 <target directory>"
	exit 1
fi
DIR=$1

mkdir -p ${DIR}/Faces.app/Contents/MacOS
mkdir -p ${DIR}/Faces.app/Contents/Resources

cat << EOF > ${DIR}/Faces.app/Contents/Info.plist
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>faces</string>
	<key>CFBundleIconFile</key>
	<string>faces.icns</string>
	<key>CFBundleIdentifier</key>
	<string>ch.tactummotum.faces</string>
	<key>NSCameraUsageDescription</key>
	<string>Faces will use the camera</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>LSUIElement</key>
	<true/>
</dict>
</plist>
EOF
go build -a -o ${DIR}/Faces.app/Contents/MacOS/faces
cp icons/faces.icns ${DIR}/Faces.app/Contents/Resources