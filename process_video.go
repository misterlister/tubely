package main

import (
	"os"
	"os/exec"
)

func processVideoForFastStart(filepath string) (string, error) {
	tempPath := filepath + ".processing"
	command := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", tempPath)
	command.Stderr = os.Stderr
	command.Stdout = os.Stdout

	if err := command.Run(); err != nil {
		return "", err
	}

	return tempPath, nil
}
