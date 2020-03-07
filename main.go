//go:generate goversioninfo -icon=icons/faces.ico versioninfo.json
package main

import (
	"bytes"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"image/color"
	"image/draw"

	"github.com/anthonynsimon/bild/blend"
	"github.com/anthonynsimon/bild/imgio"
	"github.com/gobuffalo/packr/v2"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/pkg/errors"
	"gocv.io/x/gocv"
	"golang.org/x/image/font/gofont/gobold"
)

// Constants
const (
	DefaultCapturesPerMinute = 20
	FontSize                 = 24.0
)

var (
	output = ioutil.Discard // discard output if not in debug mode
)

func main() {
	runtime.LockOSThread()

	os.Remove(errorFile())

	font, err := truetype.Parse(gobold.TTF)
	if err != nil {
		log.Fatal(errors.Wrap(err, "could not parse font"))
	}

	c, err := newConfig(findConfig(os.Args))
	if err != nil {
		log.Fatal(err)
	}

	// write to stdout for debugging
	if c.Debug {
		output = os.Stdout
	}
	log.SetOutput(output)

	if err := run(c, font); err != nil {
		_ = ioutil.WriteFile(
			errorFile(),
			[]byte(err.Error()),
			0644,
		)
		log.Fatal(err)
	}
}

func errorFile() string {
	u, err := user.Current()
	if err != nil {
		return filepath.Join(os.TempDir(), fmt.Sprintf("%s.log", filepath.Base(os.Args[0])))
	}
	return filepath.Join(u.HomeDir, fmt.Sprintf("%s.log", filepath.Base(os.Args[0])))
}

func run(c *config, font *truetype.Font) error {
	var (
		white     = color.RGBA{255, 255, 255, 255}
		rectColor = white
		textColor = white
	)

	// get the icons
	box := packr.New("assets", "./assets")
	icons, err := readIconImages(box, c)
	if err != nil {
		return err
	}

	// apply defaults if necessary
	if c.CapturesPerMinute == 0 {
		c.CapturesPerMinute = DefaultCapturesPerMinute
	}
	if c.SaveImageMax == 0 {
		c.SaveImageMax = 100
	}

	// check ImagePath
	saveImage := false
	if fi, err := os.Stat(c.SaveImagePath); err == nil && fi.IsDir() {
		saveImage = true
	}

	// open camera
	camera, err := gocv.VideoCaptureDevice(c.CameraID)
	if err != nil {
		return errors.Wrap(err, "capture device")
	}
	defer camera.Close()

	// open display window
	window := gocv.NewWindow("Face Detect (ctrl+c to quit)")
	window.SetWindowProperty(gocv.WindowPropertyAutosize, gocv.WindowAutosize)
	defer window.Close()

	// prepare image matrix
	camImage := gocv.NewMat()
	defer camImage.Close()

	fmt.Fprintf(output, "start reading camera device: %v\n", c.CameraID)
	var count int
	for {
		log.Println("new capture ...")
		if ok := camera.Read(&camImage); !ok {
			return fmt.Errorf("cannot read device %d", c.CameraID)
		}
		if camImage.Empty() {
			continue
		}
		img, err := camImage.ToImage()
		if err != nil {
			log.Println("ERROR:", errors.Wrap(err, "to image"))
			continue
		}

		detectedFaces, err := analyze(c, img)
		if err != nil {
			return err
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
			drawRect(rect, fr.Left, fr.Top, fr.Left+fr.Width, fr.Top+fr.Height, c.FrameStrenght, rectColor)

			// add gender icon
			gender := icons[strings.ToLower(f.FaceAttributes.Gender)]
			sr := gender.Bounds()
			dp := image.Point{fr.Left, fr.Top - sr.Dy() - c.FrameStrenght}
			draw.Draw(icon, image.Rectangle{dp, dp.Add(sr.Size())}, gender, sr.Min, draw.Src)

			// add glasses icon
			if f.FaceAttributes.Glasses != "NoGlasses" {
				glasses := icons[strings.ToLower(f.FaceAttributes.Glasses)]
				sr := glasses.Bounds()
				dp := image.Point{fr.Left + fr.Width - sr.Dx(), fr.Top - sr.Dy() - c.FrameStrenght}
				draw.Draw(icon, image.Rectangle{dp, dp.Add(sr.Size())}, glasses, sr.Min, draw.Src)
			}

			// add text - age and emotion
			err = addLabel(text, font, fr.Left, fr.Top+fr.Height, textColor, fmt.Sprintf("%.0f yo looks %s", f.FaceAttributes.Age, f.FaceAttributes.Emotion.String()))
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
			err := imgio.Save(path.Join(c.SaveImagePath, fmt.Sprintf("%06d.jpg", count)), res, imgio.JPEGEncoder(100))
			if err != nil {
				log.Println("ERROR:", errors.Wrap(err, "could not save image"))
			}
			if count == c.SaveImageMax {
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
			return errors.Wrap(err, "convert jpg to mat")
		}
		window.IMShow(winImage)
		if window.WaitKey(60000/c.CapturesPerMinute) == 3 {
			window.Close()
			break
		}
	}
	return nil
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
func addLabel(img draw.Image, font *truetype.Font, x, y int, c color.Color, label string) error {
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
