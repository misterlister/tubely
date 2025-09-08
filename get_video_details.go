package main

import (
	"bytes"
	"errors"
	"os/exec"

	"encoding/json"
)

type stream struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type videoDimensions struct {
	Streams []stream `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filePath,
	)

	var buffer bytes.Buffer

	cmd.Stdout = &buffer

	if err := cmd.Run(); err != nil {
		return "", err
	}

	dimensions := videoDimensions{}

	if err := json.Unmarshal(buffer.Bytes(), &dimensions); err != nil {
		return "", err
	}

	if len(dimensions.Streams) == 0 {
		return "", errors.New("no video dimensions found")
	}

	return calcAspectRatio(dimensions.Streams[0].Width, dimensions.Streams[0].Height), nil

}

func calcAspectRatio(width, height int) string {
	ratio := float64(width) / float64(height)

	if ratio > 1.7 && ratio < 1.8 {
		return LandscapeRatio
	} else if ratio > 0.55 && ratio < 0.6 {
		return PortraitRatio
	}
	return OtherRatio
}

func getAspectRatioPrefix(ratio string) string {
	if ratio == LandscapeRatio {
		return "landscape/"
	} else if ratio == PortraitRatio {
		return "portrait/"
	}
	return "other/"
}
