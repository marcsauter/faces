# Install

## Requirements
* [The Go Programming Language](https://golang.org/dl/)
* [go-task - A task runner](https://github.com/go-task/task/releases)
* [glide - Package Management for Go](https://glide.sh)
* [GoCV and OpenCV](https://gocv.io/getting-started/)

## Installation

Build faces:
```
# cd <project>
# task build
```

Edit faces configuration:
```yaml
---
cameraId: 0
subscriptionKey: [check here: https://azure.microsoft.com/en-us/services/cognitive-services/face/]
uriBase: [check here: https://azure.microsoft.com/en-us/services/cognitive-services/face/]
uriParams: ?returnFaceAttributes=age,gender,headPose,smile,facialHair,glasses,emotion,hair,makeup,occlusion,accessories,blur,exposure,noise
capturesPerMinute: 20
```

Run faces:
```
# dist/faces[.exe] config.yaml
```

## Links
* [GoCV](https://gocv.io)
* [OpenCV](https://golang.org/dl/)
* [Face API - V1.0](https://westus.dev.cognitive.microsoft.com/docs/services/563879b61984550e40cbbe8d/operations/563879b61984550f30395236)
