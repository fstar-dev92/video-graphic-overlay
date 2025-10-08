package pipeline

import (
	"bufio"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// HLSStream represents a single stream variant from the master playlist
type HLSStream struct {
	URL           string
	Bandwidth     int
	Resolution    string
	Width         int
	Height        int
	Codecs        string
	FrameRate     float64
	AverageBandwidth int
}

// HLSMasterPlaylist represents the parsed master playlist
type HLSMasterPlaylist struct {
	Streams []HLSStream
	BaseURL string
}

// parseResolution parses resolution string like "1920x1080" into width and height
func parseResolution(resolution string) (int, int) {
	parts := strings.Split(resolution, "x")
	if len(parts) != 2 {
		return 0, 0
	}
	
	width, err1 := strconv.Atoi(parts[0])
	height, err2 := strconv.Atoi(parts[1])
	
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	
	return width, height
}

// parseBandwidth parses bandwidth string and returns integer value
func parseBandwidth(bandwidth string) int {
	if bandwidth == "" {
		return 0
	}
	
	// Remove any non-numeric characters and parse
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(bandwidth, -1)
	if len(matches) == 0 {
		return 0
	}
	
	value, err := strconv.Atoi(matches[0])
	if err != nil {
		return 0
	}
	
	return value
}

// parseFrameRate parses frame rate string and returns float value
func parseFrameRate(frameRate string) float64 {
	if frameRate == "" {
		return 0
	}
	
	value, err := strconv.ParseFloat(frameRate, 64)
	if err != nil {
		return 0
	}
	
	return value
}

// ParseHLSMasterPlaylist parses an HLS master playlist and returns stream information
func ParseHLSMasterPlaylist(url string, logger *logrus.Logger) (*HLSMasterPlaylist, error) {
	logger.Infof("Parsing HLS master playlist: %s", url)
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	// Fetch the master playlist
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch master playlist: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}
	
	// Parse the playlist content
	scanner := bufio.NewScanner(resp.Body)
	var streams []HLSStream
	var currentStream HLSStream
	var baseURL string
	
	// Extract base URL for relative URLs
	urlParts := strings.Split(url, "/")
	if len(urlParts) > 0 {
		baseURL = strings.Join(urlParts[:len(urlParts)-1], "/") + "/"
	}
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			// Parse stream info line
			info := strings.TrimPrefix(line, "#EXT-X-STREAM-INF:")
			currentStream = HLSStream{}
			
			// Parse attributes
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
		} else if strings.HasPrefix(line, "http") || strings.HasPrefix(line, "/") {
			// This is a stream URL
			if currentStream.Bandwidth > 0 {
				if strings.HasPrefix(line, "/") {
					currentStream.URL = baseURL + strings.TrimPrefix(line, "/")
				} else {
					currentStream.URL = line
				}
				streams = append(streams, currentStream)
				logger.Infof("Found stream: %dx%d, %d bps, %s", 
					currentStream.Width, currentStream.Height, 
					currentStream.Bandwidth, currentStream.URL)
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading playlist: %w", err)
	}
	
	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams found in master playlist")
	}
	
	logger.Infof("Parsed %d streams from master playlist", len(streams))
	
	return &HLSMasterPlaylist{
		Streams: streams,
		BaseURL: baseURL,
	}, nil
}

// SelectBestStream selects the best quality stream based on criteria
func (h *HLSMasterPlaylist) SelectBestStream(criteria string) *HLSStream {
	if len(h.Streams) == 0 {
		return nil
	}
	
	switch strings.ToLower(criteria) {
	case "highest":
		return h.SelectHighestQuality()
	case "lowest":
		return h.SelectLowestQuality()
	case "bandwidth":
		return h.SelectByBandwidth()
	default:
		return h.SelectHighestQuality()
	}
}

// SelectHighestQuality selects the stream with the highest resolution
func (h *HLSMasterPlaylist) SelectHighestQuality() *HLSStream {
	if len(h.Streams) == 0 {
		return nil
	}
	
	// Sort by resolution (width * height), then by bandwidth
	sort.Slice(h.Streams, func(i, j int) bool {
		resolutionI := h.Streams[i].Width * h.Streams[i].Height
		resolutionJ := h.Streams[j].Width * h.Streams[j].Height
		
		if resolutionI != resolutionJ {
			return resolutionI > resolutionJ
		}
		
		return h.Streams[i].Bandwidth > h.Streams[j].Bandwidth
	})
	
	return &h.Streams[0]
}

// SelectLowestQuality selects the stream with the lowest resolution
func (h *HLSMasterPlaylist) SelectLowestQuality() *HLSStream {
	if len(h.Streams) == 0 {
		return nil
	}
	
	// Sort by resolution (width * height), then by bandwidth
	sort.Slice(h.Streams, func(i, j int) bool {
		resolutionI := h.Streams[i].Width * h.Streams[i].Height
		resolutionJ := h.Streams[j].Width * h.Streams[j].Height
		
		if resolutionI != resolutionJ {
			return resolutionI < resolutionJ
		}
		
		return h.Streams[i].Bandwidth < h.Streams[j].Bandwidth
	})
	
	return &h.Streams[0]
}

// SelectByBandwidth selects the stream with the highest bandwidth
func (h *HLSMasterPlaylist) SelectByBandwidth() *HLSStream {
	if len(h.Streams) == 0 {
		return nil
	}
	
	// Sort by bandwidth
	sort.Slice(h.Streams, func(i, j int) bool {
		return h.Streams[i].Bandwidth > h.Streams[j].Bandwidth
	})
	
	return &h.Streams[0]
}

// GetStreamByResolution finds a stream with specific resolution
func (h *HLSMasterPlaylist) GetStreamByResolution(width, height int) *HLSStream {
	for _, stream := range h.Streams {
		if stream.Width == width && stream.Height == height {
			return &stream
		}
	}
	return nil
}

// ListStreams returns all available streams sorted by quality
func (h *HLSMasterPlaylist) ListStreams() []HLSStream {
	streams := make([]HLSStream, len(h.Streams))
	copy(streams, h.Streams)
	
	// Sort by resolution (width * height), then by bandwidth
	sort.Slice(streams, func(i, j int) bool {
		resolutionI := streams[i].Width * streams[i].Height
		resolutionJ := streams[j].Width * streams[j].Height
		
		if resolutionI != resolutionJ {
			return resolutionI > resolutionJ
		}
		
		return streams[i].Bandwidth > streams[j].Bandwidth
	})
	
	return streams
}
