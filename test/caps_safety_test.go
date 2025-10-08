package test

import (
	"testing"

	"github.com/go-gst/go-gst/gst"
)

// TestCapsHandling tests that we properly handle caps to avoid GStreamer-CRITICAL errors
func TestCapsHandling(t *testing.T) {
	// Initialize GStreamer for testing
	gst.Init(nil)

	// Create caps with content
	capsWithContent := gst.NewCapsFromString("video/x-raw")
	if capsWithContent == nil {
		t.Fatal("Failed to create caps from string")
	}
	defer capsWithContent.Unref()

	// Test that caps with content have size > 0
	if capsWithContent.GetSize() == 0 {
		t.Error("Expected caps with content to have size > 0")
	}

	// Test that we can safely access structure when size > 0
	if capsWithContent.GetSize() > 0 {
		structure := capsWithContent.GetStructureAt(0)
		if structure == nil {
			t.Error("Expected to get structure from caps with content")
		} else {
			name := structure.Name()
			if name != "video/x-raw" {
				t.Errorf("Expected structure name 'video/x-raw', got '%s'", name)
			}
		}
	}

	// Test our safety pattern: only access structure if caps size > 0
	testSafetyPattern := func(caps *gst.Caps) bool {
		if caps != nil && caps.GetSize() > 0 {
			structure := caps.GetStructureAt(0)
			return structure != nil
		}
		return false
	}

	if !testSafetyPattern(capsWithContent) {
		t.Error("Safety pattern failed for valid caps")
	}

	// Test with nil caps (should not crash)
	if testSafetyPattern(nil) {
		t.Error("Safety pattern should return false for nil caps")
	}
}

// TestCapsFromString tests various caps string formats
func TestCapsFromString(t *testing.T) {
	gst.Init(nil)

	testCases := []struct {
		name        string
		capsString  string
		expectEmpty bool
	}{
		{
			name:        "Valid video caps",
			capsString:  "video/x-raw,format=I420,width=640,height=480",
			expectEmpty: false,
		},
		{
			name:        "Valid audio caps",
			capsString:  "audio/x-raw,format=S16LE,rate=44100,channels=2",
			expectEmpty: false,
		},
		{
			name:        "Simple video caps",
			capsString:  "video/x-h264",
			expectEmpty: false,
		},
		{
			name:        "Simple audio caps",
			capsString:  "audio/mpeg",
			expectEmpty: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			caps := gst.NewCapsFromString(tc.capsString)
			if caps == nil {
				t.Fatalf("Failed to create caps from string: %s", tc.capsString)
			}
			defer caps.Unref()

			isEmpty := caps.GetSize() == 0
			if isEmpty != tc.expectEmpty {
				t.Errorf("Expected empty=%v, got empty=%v for caps: %s", tc.expectEmpty, isEmpty, tc.capsString)
			}

			if !isEmpty {
				// Safe to access structure
				structure := caps.GetStructureAt(0)
				if structure == nil {
					t.Error("Expected to get structure from non-empty caps")
				}
			}
		})
	}
}
