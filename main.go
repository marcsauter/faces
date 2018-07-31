package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"image/color"
	"image/draw"
	"image/jpeg"

	"github.com/anthonynsimon/bild/blend"
	"github.com/anthonynsimon/bild/imgio"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/pkg/errors"
	"gocv.io/x/gocv"
	"golang.org/x/image/font/gofont/gobold"
	yaml "gopkg.in/yaml.v2"
)

// Constants
const (
	DefaultCapturesPerMinute = 20
	FontSize                 = 24.0
)

var (
	red       = color.RGBA{255, 0, 0, 255}
	blue      = color.RGBA{0, 0, 255, 255}
	green     = color.RGBA{0, 255, 0, 255}
	yellow    = color.RGBA{255, 255, 0, 255}
	white     = color.RGBA{255, 255, 255, 255}
	black     = color.RGBA{0, 0, 0, 255}
	rectColor = white
	textColor = white
	ttf       = gobold.TTF
	font      *truetype.Font
	saveImage bool
)

type configuration struct {
	CameraID            int    `yaml:"cameraId"`
	SubscriptionKey     string `yaml:"subscriptionKey"`
	URIBase             string `yaml:"uriBase"`
	URIParams           string `yaml:"uriParams"`
	CapturesPerMinute   int    `yaml:"capturesPerMinute"`
	FrameStrenght       int    `yaml:"frameStrenght"`
	SaveImagePath       string `yaml:"saveImagePath"`
	SaveImageMax        int    `yaml:"saveImageMax"`
	IconMale            string `yaml:"iconMale"`
	IconFemale          string `yaml:"iconFemale"`
	IconReadingGlasses  string `yaml:"iconReadingGlasses"`
	IconSunGlasses      string `yaml:"iconSunGlasses"`
	IconSwimmingGoggles string `yaml:"iconSwimmingGoggles"`
}

func init() {
	var err error
	font, err = truetype.Parse(ttf)
	if err != nil {
		log.Fatal(errors.Wrap(err, "could not parse font"))
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s config.yaml\n", filepath.Base(os.Args[0]))
		return
	}

	// read configuration
	cfg := configuration{}
	data, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatal(err)
	}

	icons, err := readIconImages(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	// apply defaults if necessary
	if cfg.CapturesPerMinute == 0 {
		cfg.CapturesPerMinute = DefaultCapturesPerMinute
	}
	if cfg.SaveImageMax == 0 {
		cfg.SaveImageMax = 100
	}

	// check ImagePath
	if fi, err := os.Stat(cfg.SaveImagePath); err == nil && fi.IsDir() {
		saveImage = true
	}

	// open camera
	camera, err := gocv.VideoCaptureDevice(cfg.CameraID)
	if err != nil {
		log.Fatal(errors.Wrap(err, "capture device"))
	}
	defer camera.Close()

	// open display window
	window := gocv.NewWindow("Face Detect")
	defer window.Close()

	// prepare image matrix
	camImage := gocv.NewMat()
	defer camImage.Close()

	fmt.Printf("start reading camera device: %v\n", cfg.CameraID)
	var count int
	for {
		log.Println("new capture ...")
		if ok := camera.Read(&camImage); !ok {
			fmt.Printf("cannot read device %d\n", cfg.CameraID)
			return
		}
		if camImage.Empty() {
			continue
		}
		img, err := camImage.ToImage()
		if err != nil {
			log.Println("ERROR:", errors.Wrap(err, "to image"))
			continue
		}

		detectedFaces, err := analyze(&cfg, img)
		if err != nil {
			log.Println("ERROR:", err)
			continue
		}

		bg := img.(*image.RGBA)
		rect := image.NewRGBA(bg.Bounds())
		icon := image.NewRGBA(bg.Bounds())
		text := image.NewRGBA(bg.Bounds())

		timer := time.NewTimer(time.Duration(60/cfg.CapturesPerMinute) * time.Second)
		// draw a rectangle around each face on the original image along with informations
		for _, f := range detectedFaces {
			fr := f.FaceRectangle

			// draw rectangle around detected face
			drawRect(rect, fr.Left, fr.Top, fr.Left+fr.Width, fr.Top+fr.Height, cfg.FrameStrenght, rectColor)

			// add gender icon
			gender := icons[strings.ToLower(f.FaceAttributes.Gender)]
			sr := gender.Bounds()
			dp := image.Point{fr.Left, fr.Top - sr.Dy() - cfg.FrameStrenght}
			draw.Draw(icon, image.Rectangle{dp, dp.Add(sr.Size())}, gender, sr.Min, draw.Src)

			// add glasses icon
			if f.FaceAttributes.Glasses != "NoGlasses" {
				glasses := icons[strings.ToLower(f.FaceAttributes.Glasses)]
				sr := glasses.Bounds()
				dp := image.Point{fr.Left + fr.Width - sr.Dx(), fr.Top - sr.Dy() - cfg.FrameStrenght}
				draw.Draw(icon, image.Rectangle{dp, dp.Add(sr.Size())}, glasses, sr.Min, draw.Src)
			}

			// add text - age and emotion
			err = addLabel(text, fr.Left, fr.Top+fr.Height, textColor, fmt.Sprintf("%.0f yo looks %s", f.FaceAttributes.Age, f.FaceAttributes.Emotion.String()))
			if err != nil {
				log.Println("ERROR:", errors.Wrap(err, "could not add label"))
			}

			// print to console
			fmt.Printf("%.0f yo %s with %s looks %s\n", f.FaceAttributes.Age, f.FaceAttributes.Gender, f.FaceAttributes.Glasses, f.FaceAttributes.Emotion.String())
		}

		// put layers together
		var res image.Image
		res = blend.Add(bg, rect)     // rectangle layer
		res = blend.Normal(res, icon) // icon layer
		res = blend.Add(res, text)    // text layer

		// save image
		if saveImage && len(detectedFaces) > 0 {
			count++
			err := imgio.Save(path.Join(cfg.SaveImagePath, fmt.Sprintf("%06d.jpg", count)), res, imgio.JPEG)
			if err != nil {
				log.Println("ERROR:", errors.Wrap(err, "could not save image"))
			}
			if count == cfg.SaveImageMax {
				count = 0
			}
		}

		// show image
		winImage, err := gocv.ImageToMatRGBA(res)
		if err != nil {
			log.Fatal(errors.Wrap(err, "convert jpg to mat"))
		}
		window.IMShow(winImage)
		if window.WaitKey(1) >= 0 {
			break
		}
		<-timer.C
	}
}

func readIconImages(cfg *configuration) (map[string]image.Image, error) {
	icons := make(map[string]image.Image)

	var r io.Reader
	var err error

	r, err = os.Open(cfg.IconMale)
	if err != nil {
		return nil, errors.Wrapf(err, "read icon %s", cfg.IconMale)
	}
	icons["male"], _, err = image.Decode(r)
	if err != nil {
		return nil, errors.Wrapf(err, "decode icon %s", cfg.IconMale)
	}

	r, err = os.Open(cfg.IconFemale)
	if err != nil {
		return nil, errors.Wrapf(err, "read icon %s", cfg.IconFemale)
	}
	icons["female"], _, err = image.Decode(r)
	if err != nil {
		return nil, errors.Wrapf(err, "decode icon %s", cfg.IconFemale)
	}

	r, err = os.Open(cfg.IconReadingGlasses)
	if err != nil {
		return nil, errors.Wrapf(err, "read icon %s", cfg.IconReadingGlasses)
	}
	icons["readingglasses"], _, err = image.Decode(r)
	if err != nil {
		return nil, errors.Wrapf(err, "decode icon %s", cfg.IconReadingGlasses)
	}

	r, err = os.Open(cfg.IconSunGlasses)
	if err != nil {
		return nil, errors.Wrapf(err, "read icon %s", cfg.IconSunGlasses)
	}
	icons["sunglasses"], _, err = image.Decode(r)
	if err != nil {
		return nil, errors.Wrapf(err, "decode icon %s", cfg.IconSunGlasses)
	}

	r, err = os.Open(cfg.IconSwimmingGoggles)
	if err != nil {
		return nil, errors.Wrapf(err, "read icon %s", cfg.IconSwimmingGoggles)
	}
	icons["swimminggoggles"], _, err = image.Decode(r)
	if err != nil {
		return nil, errors.Wrapf(err, "decode icon %s", cfg.IconSwimmingGoggles)
	}

	return icons, nil
}

// analyze image using Face API
func analyze(cfg *configuration, img image.Image) (faces, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", cfg.URIBase+cfg.URIParams, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/octet-stream")
	req.Header.Add("Ocp-Apim-Subscription-Key", cfg.SubscriptionKey)

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

// drawHLine draws a horizontal line
func drawHLine(img draw.Image, x1, y, x2, thick int, c color.Color) {
	for ; x1 <= x2; x1++ {
		img.Set(x1, y, c)
		for t := 0; t < thick; t++ {
			img.Set(x1, y-t, c)
		}
	}
}

// drawVLine draws a veritcal line
func drawVLine(img draw.Image, x, y1, y2, thick int, c color.Color) {
	for ; y1 <= y2; y1++ {
		img.Set(x, y1, c)
		for t := 0; t < thick; t++ {
			img.Set(x-t, y1, c)
		}
	}
}

// drawRect draws a rectangle with drawHLine and drawVLine
func drawRect(img draw.Image, x1, y1, x2, y2, thick int, c color.Color) {
	drawHLine(img, x1, y1, x2, thick, c)
	drawHLine(img, x1, y2, x2, thick, c)
	drawVLine(img, x1, y1, y2, thick, c)
	drawVLine(img, x2, y1, y2, thick, c)
}

// addLabel
func addLabel(img draw.Image, x, y int, c color.Color, label string) error {
	fc := freetype.NewContext()
	fc.SetSrc(image.NewUniform(c))
	fc.SetDst(img)
	fc.SetFont(font)
	fc.SetFontSize(FontSize)
	fc.SetClip(img.Bounds())
	fc.SetDPI(72)
	pt := freetype.Pt(x, y+int(fc.PointToFixed(FontSize)>>6))
	_, err := fc.DrawString(label, pt)
	return err
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
	return fmt.Sprintf("%s", res)
}

type apiError struct {
	Error errorDetails `json:"error"`
}

type errorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
