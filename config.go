package main

import (
	"io/ioutil"
	"net/url"
	"os"
	"os/user"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

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
	var configFileNames = []string{
		"faces.yaml",
		"faces.yml",
		".faces.yaml",
		".faces.yml",
	}

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
