package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"gocv.io/x/gocv"
)

const (
	subscriptionKey = "8d2d3fb70c3e4540aec1cd59534709c2"
	uriBase         = "https://westcentralus.api.cognitive.microsoft.com/face/v1.0/detect"
	params          = "?returnFaceAttributes=age,gender,headPose,smile,facialHair," +
		"glasses,emotion,hair,makeup,occlusion,accessories,blur,exposure,noise"
	uri               = uriBase + params
	capturesPerMinute = 20
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s [camera ID]\n", filepath.Base(os.Args[0]))
		return
	}

	// parse args
	deviceID, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	// open webcam
	webcam, err := gocv.VideoCaptureDevice(int(deviceID))
	if err != nil {
		log.Fatal(err)
	}
	defer webcam.Close()

	// open display window
	window := gocv.NewWindow("Face Detect")
	defer window.Close()

	// prepare image matrix
	img := gocv.NewMat()
	defer img.Close()

	// color for the rect when faces detected
	blue := color.RGBA{0, 0, 255, 0}

	fmt.Printf("start reading camera device: %v\n", deviceID)
	for {
		log.Println("new capture ...")
		if ok := webcam.Read(&img); !ok {
			fmt.Printf("cannot read device %d\n", deviceID)
			return
		}
		if img.Empty() {
			continue
		}

		timer := time.NewTimer(time.Duration(60/capturesPerMinute) * time.Second)

		detectedFaces, err := analyze(&img)
		if err != nil {
			log.Println("ERROR:", err)
		}

		// draw a rectangle around each face on the original image,
		// along with text informations
		for _, f := range detectedFaces {
			r := f.FaceRectangle
			gocv.Rectangle(&img, image.Rect(r.Left, r.Top, r.Left+r.Width, r.Top+r.Height), blue, 3)
			a := f.FaceAttributes
			text := fmt.Sprintf("%d years old %s, wearing %s and looks %s", int(a.Age), a.Gender, a.Glasses, a.Emotion)
			//size := gocv.GetTextSize("Human", gocv.FontHersheyPlain, 1.2, 2)
			pt := image.Pt(r.Left, r.Top-5)
			gocv.PutText(&img, text, pt, gocv.FontHersheyPlain, 1.2, blue, 2)
			fmt.Println("> face detected:", text, f.FaceID) // write to stdout
		}

		window.IMShow(img)
		if window.WaitKey(1) >= 0 {
			break
		}
		<-timer.C
	}
}

func analyze(img *gocv.Mat) (faces, error) {

	png, err := gocv.IMEncode(gocv.JPEGFileExt, *img)
	if err != nil {
		return nil, errors.Wrap(err, "IMEncode failed")
	}

	req, err := http.NewRequest("POST", uri, bytes.NewReader(png))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/octet-stream")
	req.Header.Add("Ocp-Apim-Subscription-Key", subscriptionKey)

	/*
		requestDump, err := httputil.DumpRequest(req, false)
		if err != nil {
			log.Println(errors.Wrap(err, "httputil.DumpRequest"))
		}
		fmt.Println(string(requestDump))
	*/

	client := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "client.Do")
	}
	defer resp.Body.Close()

	/*
		responseDump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			log.Println(errors.Wrap(err, "httputil.DumpResponse"))
		}
		fmt.Println(string(responseDump))
	*/

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		e := apiError{}
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, errors.Wrap(err, "unmarshal api error failed")
		}
		return nil, fmt.Errorf("%s", e.Error.Message)
	}

	detectedFaces := faces{}
	err = json.Unmarshal(data, &detectedFaces)
	return detectedFaces, nil
}

// Face API V1.0 response data model
// https://westus.dev.cognitive.microsoft.com/docs/services/563879b61984550e40cbbe8d/operations/563879b61984550f30395236

type faces []face

type face struct {
	FaceAttributes attributes `json:"faceAttributes"`
	FaceID         string     `json:"faceId"`
	FaceRectangle  rectangle  `json:"faceRectangle"`
}

type attributes struct {
	Age     float32 `json:"age"`
	Gender  string  `json:"gender"`
	Glasses string  `json:"glasses"`
	Emotion emotion `json:"emotion"`
}

type rectangle struct {
	Height int `json:"height"`
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
}

type emotion struct {
	Anger     float32 `json:"anger"`
	Contempt  float32 `json:"contempt"`
	Disgust   float32 `json:"disgust"`
	Fear      float32 `json:"fear"`
	Happiness float32 `json:"happiness"`
	Neutral   float32 `json:"neutral"`
	Sadness   float32 `json:"sadness"`
	Surprise  float32 `json:"surprise"`
}

func (e emotion) String() string {
	m := make(map[string]float32)
	m["angry"] = e.Anger
	m["contemptuous"] = e.Contempt
	m["disgusted"] = e.Disgust
	m["feared"] = e.Fear
	m["happy"] = e.Happiness
	m["neutral"] = e.Neutral
	m["sad"] = e.Sadness
	m["surprised"] = e.Surprise
	var max float32
	var res string
	for k, v := range m {
		if v > max {
			res = k
			max = v
		}
	}
	return fmt.Sprintf("%s (%f)", res, m[res])
}

type apiError struct {
	Error errorDetails `json:"error"`
}

type errorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
