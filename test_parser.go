package main

import (
	"fmt"
	"strings"

	"video-graphic-overlay-gstreamer/internal/pipeline"
)

func main() {
	// Your master playlist content
	masterPlaylistContent := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-STREAM-INF:AVERAGE-BANDWIDTH=1393000,BANDWIDTH=1566000,CODECS="avc1.640029,mp4a.40.5",RESOLUTION=720x404,FRAME-RATE=30.000,GROUP-ID="aac"
https://hls.showfer.com/live/KiioJ/Jacob-AD/1200/720x404/adstorm.m3u8
#EXT-X-STREAM-INF:AVERAGE-BANDWIDTH=398000,BANDWIDTH=498000,CODECS="avc1.640029,mp4a.40.5",RESOLUTION=384x216,FRAME-RATE=30.000,GROUP-ID="aac"
https://hls.showfer.com/live/KiioJ/Jacob-AD/230/384x216/adstorm.m3u8
#EXT-X-STREAM-INF:AVERAGE-BANDWIDTH=613000,BANDWIDTH=767000,CODECS="avc1.640029,mp4a.40.5",RESOLUTION=384x216,FRAME-RATE=30.000,GROUP-ID="aac"
https://hls.showfer.com/live/KiioJ/Jacob-AD/440/384x216/adstorm.m3u8
#EXT-X-STREAM-INF:AVERAGE-BANDWIDTH=879000,BANDWIDTH=1053000,CODECS="avc1.640029,mp4a.40.5",RESOLUTION=512x288,FRAME-RATE=30.000,GROUP-ID="aac"
https://hls.showfer.com/live/KiioJ/Jacob-AD/700/512x288/adstorm.m3u8
#EXT-X-STREAM-INF:AVERAGE-BANDWIDTH=2719000,BANDWIDTH=2884000,CODECS="avc1.640029,mp4a.40.5",RESOLUTION=1280x720,FRAME-RATE=30.000,GROUP-ID="aac"
https://hls.showfer.com/live/KiioJ/Jacob-AD/2500/1280x720/adstorm.m3u8
#EXT-X-STREAM-INF:AVERAGE-BANDWIDTH=5256000,BANDWIDTH=5470000,CODECS="avc1.640029,mp4a.40.5",RESOLUTION=1920x1080,FRAME-RATE=30.000,GROUP-ID="aac"
https://hls.showfer.com/live/KiioJ/Jacob-AD/5000/1920x1080/adstorm.m3u8`

	fmt.Println("Parsing master playlist content...")
	fmt.Println("==================================")

	// Parse the content manually to demonstrate
	lines := strings.Split(masterPlaylistContent, "\n")
	var streams []pipeline.HLSStream
	var currentStream pipeline.HLSStream

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			// Parse stream info
			info := strings.TrimPrefix(line, "#EXT-X-STREAM-INF:")
			currentStream = pipeline.HLSStream{}
			
			attrs := strings.Split(info, ",")
			for _, attr := range attrs {
				parts := strings.SplitN(attr, "=", 2)
				if len(parts) != 2 {
					continue
				}
				
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				
				switch key {
				case "BANDWIDTH":
					currentStream.Bandwidth = parseBandwidth(value)
				case "AVERAGE-BANDWIDTH":
					currentStream.AverageBandwidth = parseBandwidth(value)
				case "RESOLUTION":
					currentStream.Resolution = value
					currentStream.Width, currentStream.Height = parseResolution(value)
				case "CODECS":
					currentStream.Codecs = value
				case "FRAME-RATE":
					currentStream.FrameRate = parseFrameRate(value)
				}
			}
		} else if strings.HasPrefix(line, "https://") {
			currentStream.URL = line
			streams = append(streams, currentStream)
		}
	}

	fmt.Printf("Found %d streams:\n\n", len(streams))
	
	for i, stream := range streams {
		fmt.Printf("%d. %dx%d @ %d bps (avg: %d bps)\n", 
			i+1, stream.Width, stream.Height, 
			stream.Bandwidth, stream.AverageBandwidth)
		fmt.Printf("   URL: %s\n\n", stream.URL)
	}

	// Test selection
	fmt.Println("Stream selection:")
	fmt.Println("================")
	
	// Highest quality (by resolution)
	highest := streams[0]
	for _, stream := range streams {
		if (stream.Width * stream.Height) > (highest.Width * highest.Height) {
			highest = stream
		}
	}
	fmt.Printf("Highest quality: %dx%d @ %d bps\n", 
		highest.Width, highest.Height, highest.Bandwidth)
	fmt.Printf("URL: %s\n\n", highest.URL)

	// Highest bandwidth
	highestBW := streams[0]
	for _, stream := range streams {
		if stream.Bandwidth > highestBW.Bandwidth {
			highestBW = stream
		}
	}
	fmt.Printf("Highest bandwidth: %dx%d @ %d bps\n", 
		highestBW.Width, highestBW.Height, highestBW.Bandwidth)
	fmt.Printf("URL: %s\n\n", highestBW.URL)
}

func parseBandwidth(bandwidth string) int {
	if bandwidth == "" {
		return 0
	}
	
	value := 0
	fmt.Sscanf(bandwidth, "%d", &value)
	return value
}

func parseResolution(resolution string) (int, int) {
	var width, height int
	fmt.Sscanf(resolution, "%dx%d", &width, &height)
	return width, height
}

func parseFrameRate(frameRate string) float64 {
	var rate float64
	fmt.Sscanf(frameRate, "%f", &rate)
	return rate
}
