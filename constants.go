package main

const (
	MaxVideoMemory   = 1 << 30
	MaxImageMemory   = 1 << 20
	FilenameByteSize = 32
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
