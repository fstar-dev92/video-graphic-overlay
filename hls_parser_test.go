package main

import (
	"fmt"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"video-graphic-overlay-gstreamer/internal/pipeline"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run hls_parser_test.go <master_playlist_url>")
		fmt.Println("Example: go run hls_parser_test.go https://hls.showfer.com/live/KiioJ/Jacob-AD/adstorm.m3u8")
		os.Exit(1)
	}

	url := os.Args[1]
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	fmt.Printf("Parsing HLS master playlist: %s\n\n", url)

	// Parse the master playlist
	playlist, err := pipeline.ParseHLSMasterPlaylist(url, logger)
	if err != nil {
		log.Fatalf("Failed to parse master playlist: %v", err)
	}

	// List all available streams
	fmt.Println("Available streams:")
	fmt.Println("==================")
	streams := playlist.ListStreams()
	for i, stream := range streams {
		fmt.Printf("%d. %dx%d @ %d bps (avg: %d bps) - %s\n", 
			i+1, stream.Width, stream.Height, 
			stream.Bandwidth, stream.AverageBandwidth, stream.URL)
	}

	fmt.Println("\nStream selection tests:")
	fmt.Println("======================")

	// Test different selection criteria
	criteria := []string{"highest", "lowest", "bandwidth"}
	for _, criterion := range criteria {
		selected := playlist.SelectBestStream(criterion)
		if selected != nil {
			fmt.Printf("%s: %dx%d @ %d bps - %s\n", 
				criterion, selected.Width, selected.Height, 
				selected.Bandwidth, selected.URL)
		}
	}

	// Test specific resolution selection
	fmt.Println("\nSpecific resolution tests:")
	fmt.Println("==========================")
	resolutions := [][]int{{1920, 1080}, {1280, 720}, {720, 404}, {512, 288}, {384, 216}}
	for _, res := range resolutions {
		stream := playlist.GetStreamByResolution(res[0], res[1])
		if stream != nil {
			fmt.Printf("%dx%d: %d bps - %s\n", 
				stream.Width, stream.Height, stream.Bandwidth, stream.URL)
		} else {
			fmt.Printf("%dx%d: Not available\n", res[0], res[1])
		}
	}
}
