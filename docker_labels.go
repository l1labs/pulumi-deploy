package deploy

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"
)

//DockerLabelExtractor reads and extracts labels from the provided Docker file
type DockerLabelExtractor struct {
	Path string
}

func (e *DockerLabelExtractor) validate() error {
	if e.Path == "" {
		return errors.New("Path cannot be empty")
	}
	_, err := os.Open(e.Path)
	if err != nil {
		return err
	}

	return nil
}

// Extract returns the labels inside of a Dockerfile
func (e *DockerLabelExtractor) Extract() (map[string]string, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(e.Path)
	if err != nil {
		return nil, err
	}
	fileData := string(data)
	lines := strings.Split(fileData, "\n")

	labels := map[string]string{}
	for _, line := range lines {
		if strings.HasPrefix(line, "LABEL ") {
			label := strings.TrimLeft(line, "LABEL ")
			components := strings.SplitN(label, "=", 2)
			if len(components) < 2 {
				continue
			}
			labels[components[0]] = strings.Trim(components[1], `"`)
		}
	}

	return labels, nil
}
