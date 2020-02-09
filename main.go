//go:generate goversioninfo -icon=icons/faces.ico versioninfo.json
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"image/color"
	"image/draw"
	"image/jpeg"

	"github.com/anthonynsimon/bild/blend"
	"github.com/anthonynsimon/bild/imgio"
	"github.com/gobuffalo/packr/v2"
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
	configFileNames = []string{
		"faces.yaml",
		"faces.yml",
		".faces.yaml",
		".faces.yml",
	}
	output    = ioutil.Discard // discard output if not in debug mode
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

// I know init is not cool ...
func init() {
	var err error
	font, err = truetype.Parse(ttf)
	if err != nil {
		log.Fatal(errors.Wrap(err, "could not parse font"))
	}
}

// configuration for faces app
type config struct {
	CameraID          int    `yaml:"cameraId"`
	SubscriptionKey   string `yaml:"subscriptionKey"`
	URIBase           string `yaml:"uriBase"`
	URIPath           string `yaml:"uriPath"`
	URIParams         string `yaml:"uriParams"`
	uri               string
	CapturesPerMinute int    `yaml:"capturesPerMinute"`
	FrameStrenght     int    `yaml:"frameStrenght"`
	SaveImagePath     string `yaml:"saveImagePath"`
	SaveImageMax      int    `yaml:"saveImageMax"`
	Debug             bool   `yaml:"debug"`
}

func newConfig(file string) (*config, error) {
	// read configuration
	c := config{}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	u, err := url.Parse(c.URIBase)
	if err != nil {
		return nil, err
	}
	u.Path = c.URIPath
	u.RawQuery = c.URIParams
	c.uri = u.String()
	return &c, nil
}

func (c *config) URI() string {
	return c.uri
}

// findConfig in different locations, first match will be returned
func findConfig(args []string) string {
	dirs := []string{}
	// check args
	if len(args) > 1 && args[1] != "" {
		return args[1]
	}
	// get user home
	u, _ := user.Current()
	if u != nil {
		dirs = append(dirs, u.HomeDir)
	}
	// get executable path
	e, _ := os.Executable()
	if e != "" {
		ep := filepath.Dir(e)
		dirs = append(dirs, ep, filepath.Join(ep, "..", "Resources"))
	}
	for _, d := range dirs {
		for _, c := range configFileNames {
			n := filepath.Join(d, c)
			if _, err := os.Stat(n); err == nil {
				return n
			}
		}
	}
	return ""
}

func main() {
	runtime.LockOSThread()

	cfg, err := newConfig(findConfig(os.Args))
	if err != nil {
		log.Fatal(err)
	}

	// write to stdout for debugging
	if cfg.Debug {
		output = os.Stdout
	}
	log.SetOutput(output)

	// get the icons
	box := packr.New("assets", "./assets")
	icons, err := readIconImages(box, cfg)
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
	window := gocv.NewWindow("Face Detect (ctrl+c to quit)")
	window.SetWindowProperty(gocv.WindowPropertyAutosize, gocv.WindowAutosize)
	defer window.Close()

	// prepare image matrix
	camImage := gocv.NewMat()
	defer camImage.Close()

	fmt.Fprintf(output, "start reading camera device: %v\n", cfg.CameraID)
	var count int
	for {
		log.Println("new capture ...")
		if ok := camera.Read(&camImage); !ok {
			log.Fatal(fmt.Errorf("cannot read device %d", cfg.CameraID))
		}
		if camImage.Empty() {
			continue
		}
		img, err := camImage.ToImage()
		if err != nil {
			log.Println("ERROR:", errors.Wrap(err, "to image"))
			continue
		}

		detectedFaces, err := analyze(cfg, img)
		if err != nil {
			log.Println("ERROR:", err)
			continue
		}

		bg := img.(*image.RGBA)
		rect := image.NewRGBA(bg.Bounds())
		icon := image.NewRGBA(bg.Bounds())
		text := image.NewRGBA(bg.Bounds())

		// draw a rectangle around each face on the original image along with informations
		for _, f := range detectedFaces {
			//
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
			fmt.Fprintf(output, "%.0f yo %s with %s looks %s\n", f.FaceAttributes.Age, f.FaceAttributes.Gender, f.FaceAttributes.Glasses, f.FaceAttributes.Emotion.String())
		}

		// put layers together
		var res image.Image
		res = blend.Add(bg, rect)     // rectangle layer
		res = blend.Normal(res, icon) // icon layer
		res = blend.Add(res, text)    // text layer

		// save image
		if saveImage && len(detectedFaces) > 0 {
			count++
			err := imgio.Save(path.Join(cfg.SaveImagePath, fmt.Sprintf("%06d.jpg", count)), res, imgio.JPEGEncoder(100))
			if err != nil {
				log.Println("ERROR:", errors.Wrap(err, "could not save image"))
			}
			if count == cfg.SaveImageMax {
				count = 0
			}
		}

		// no faces detected
		if len(detectedFaces) == 0 {
			fmt.Fprintf(output, "no faces detected\n")
		}

		// show image
		winImage, err := gocv.ImageToMatRGBA(res)
		if err != nil {
			log.Fatal(errors.Wrap(err, "convert jpg to mat"))
		}
		window.IMShow(winImage)
		if window.WaitKey(60000/cfg.CapturesPerMinute) == 3 {
			window.Close()
			os.Exit(0)
		}
	}
}

// readIconImages from box
func readIconImages(box *packr.Box, cfg *config) (map[string]image.Image, error) {
	icons := make(map[string]image.Image)

	d, err := box.Find("male.jpg")
	if err != nil {
		return nil, err
	}
	icons["male"], _, err = image.Decode(bytes.NewReader(d))
	if err != nil {
		return nil, err
	}

	d, err = box.Find("female.jpg")
	if err != nil {
		return nil, err
	}
	icons["female"], _, err = image.Decode(bytes.NewReader(d))
	if err != nil {
		return nil, err
	}

	d, err = box.Find("readingglasses.jpg")
	if err != nil {
		return nil, err
	}
	icons["readingglasses"], _, err = image.Decode(bytes.NewReader(d))
	if err != nil {
		return nil, err
	}

	d, err = box.Find("sunglasses.jpg")
	if err != nil {
		return nil, err
	}
	icons["sunglasses"], _, err = image.Decode(bytes.NewReader(d))
	if err != nil {
		return nil, err
	}

	d, err = box.Find("swimminggoggles.jpg")
	if err != nil {
		return nil, err
	}
	icons["swimminggoggles"], _, err = image.Decode(bytes.NewReader(d))
	if err != nil {
		return nil, err
	}

	return icons, nil
}

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
	return res
}

type apiError struct {
	Error errorDetails `json:"error"`
}

type errorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
