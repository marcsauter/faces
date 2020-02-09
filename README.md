# Install

## Requirements
* [The Go Programming Language](https://golang.org/dl/)
* [GoCV and OpenCV](https://gocv.io/getting-started/)

## Installation

### Embed icons
> Icon size should be around 50x50 pixels

- download/install [Packr (v2)](https://github.com/gobuffalo/packr/tree/master/v2)
- replace the icons in `assets`directory (keep the names)
```bash
# packr2
```

### Build faces
```bash
# cd <project>
# go build -a -o <target directory>/faces[.exe]
```

### Build Windows App
> Write a "GUI binary" instead of a "console binary"
```bash
# cd <project>
# go build -a -ldflags "-H windowsgui" -o <target directory>/faces.exe
```

### Build macOS App

To build you App on your Desktop enter:
```bash
# scripts/macos.app.sh ${HOME}/Desktop
```
> The app icon may not be displayed properly. Be patient or restart. You can also try [this](https://apple.stackexchange.com/questions/280877/apps-icons-not-appearing).


### Edit faces configuration
> [Here](config/example.yaml) you will find an example configuration

> The config file `faces.yaml` | `faces.yml` | `.faces.yaml` | `.faces.yml` should be in your `${HOME}` directory. Otherwise you have to enter it as an argument.

```yaml
---
cameraId: 0
subscriptionKey: [check here: https://azure.microsoft.com/en-us/services/cognitive-services/face/]
uriBase: [check here: https://azure.microsoft.com/en-us/services/cognitive-services/face/]
uriPath: 
uriParams: ?returnFaceAttributes=age,gender,headPose,smile,facialHair,glasses,emotion,hair,makeup,occlusion,accessories,blur,exposure,noise
...
```


## Run faces
```bash
# dist/faces[.exe] [faces.yaml]
```

## Links
* [Golang](https://golang.org/dl/)
* [GoCV](https://gocv.io)
* [Face documentation](https://docs.microsoft.com/en-us/azure/cognitive-services/face/)
