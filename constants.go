package main

const (
	MaxVideoMemory   = 1 << 30
	MaxImageMemory   = 1 << 20
	FilenameByteSize = 32
	LandscapeRatio   = "16:9"
	PortraitRatio    = "9:16"
	OtherRatio       = "other"
)

var ValidImageTypes = []string{
	"png",
	"jpg",
	"webp",
	"gif",
}

var ValidVideoTypes = []string{
	"mp4",
}
