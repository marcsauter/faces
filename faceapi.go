package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/pkg/errors"
)

// analyze image using Face API
func analyze(cfg *config, img image.Image) (faces, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode jpeg")
	}

	req, err := http.NewRequest("POST", cfg.URI(), &buf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.Header.Add("Content-Type", "application/octet-stream")
	req.Header.Add("Ocp-Apim-Subscription-Key", cfg.SubscriptionKey)

	requestDump, err := httputil.DumpRequest(req, false)
	if err != nil {
		log.Println(errors.Wrap(err, "httputil.DumpRequest"))
	}
	fmt.Fprintln(output, string(requestDump))

	client := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "client.Do")
	}
	defer resp.Body.Close()

	responseDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		log.Println(errors.Wrap(err, "httputil.DumpResponse"))
	}
	fmt.Fprintln(output, string(responseDump))

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read body")
	}

	if resp.StatusCode != 200 {
		e := apiError{}
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, errors.Wrap(err, "unmarshal api error failed")
		}
		return nil, fmt.Errorf("API %s", e.Error.Message)
	}

	detectedFaces := faces{}
	err = json.Unmarshal(data, &detectedFaces)
	return detectedFaces, err
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
	return res
}

type apiError struct {
	Error errorDetails `json:"error"`
}

type errorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
