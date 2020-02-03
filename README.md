# Install

## Requirements
* [The Go Programming Language](https://golang.org/dl/)
* [GoCV and OpenCV](https://gocv.io/getting-started/)

## Installation

Build faces:
```
# cd <project>
# go build -a -o <target directory>/faces
```

Edit faces configuration:
```yaml
---
cameraId: 0
subscriptionKey: [check here: https://azure.microsoft.com/en-us/services/cognitive-services/face/]
uriBase: [check here: https://azure.microsoft.com/en-us/services/cognitive-services/face/]
uriParams: ?returnFaceAttributes=age,gender,headPose,smile,facialHair,glasses,emotion,hair,makeup,occlusion,accessories,blur,exposure,noise
...
```
> Icon size should be around 50x50 pixels

Run faces:
```
# dist/faces[.exe] faces.yaml
```

## Links
* [Golang](https://golang.org/dl/)
* [GoCV](https://gocv.io)
* [Face documentation](https://docs.microsoft.com/en-us/azure/cognitive-services/face/)
