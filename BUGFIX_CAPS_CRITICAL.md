# Bug Fix: GStreamer-CRITICAL Caps Assertion Error

## Problem

When using `playbin3` or `urisourcebin` source types, the application was generating GStreamer-CRITICAL errors:

```
(main:559660): GStreamer-CRITICAL **: 13:50:42.453: gst_caps_get_structure: assertion 'index < GST_CAPS_LEN (caps)' failed
INFO[2025-10-08T13:50:42-04:00] Urisourcebin new pad added: src_1            
(main:559660): GStreamer-CRITICAL **: 13:50:42.453: gst_caps_get_structure: assertion 'index < GST_CAPS_LEN (caps)' failed
```

## Root Cause

The error occurred in the pad-added callbacks when trying to access the first structure of caps that had no structures (empty caps). The code was calling:

```go
structure := caps.GetStructureAt(0)
```

Without first checking if the caps contained any structures. When `caps.GetSize()` returns 0, calling `GetStructureAt(0)` triggers the GStreamer assertion failure.

## Solution

Added a safety check to verify caps contain structures before accessing them:

### Before (Problematic Code)
```go
if caps != nil {
    structure := caps.GetStructureAt(0)  // CRITICAL error if caps is empty
    if structure != nil {
        mediaType := structure.Name()
        // ... process media type
    }
}
```

### After (Fixed Code)
```go
if caps != nil && caps.GetSize() > 0 {  // Added size check
    structure := caps.GetStructureAt(0)  // Now safe to call
    if structure != nil {
        mediaType := structure.Name()
        // ... process media type
    }
}
```

## Files Modified

The fix was applied to all pad-added callbacks in `internal/pipeline/pipeline.go`:

1. **linkSouphttpsrcElements()** - HLS demux and TS demux callbacks
2. **linkPlaybin3Elements()** - Playbin3 pad-added callback  
3. **linkUrisourcebinElements()** - Urisourcebin pad-added callback
4. **linkVideoChain()** - Video decoder callback
5. **linkAudioChain()** - Audio decoder callback

## Enhanced Error Logging

Also improved error logging to distinguish between different failure cases:

```go
} else {
    if caps != nil {
        p.logger.Warnf("Empty capabilities for %s pad %s (caps size: %d)", 
                      sourceType, padName, caps.GetSize())
    } else {
        p.logger.Warnf("Could not get capabilities for %s pad %s", 
                      sourceType, padName)
    }
}
```

This provides better debugging information when caps are empty vs. when caps retrieval fails entirely.

## Testing

The fix ensures:

1. ✅ No more GStreamer-CRITICAL assertion errors
2. ✅ Proper handling of empty caps from any source
3. ✅ Better error logging for debugging
4. ✅ All source types (souphttpsrc, playbin3, urisourcebin) work correctly
5. ✅ Backward compatibility maintained

## Impact

- **Severity**: High (was causing critical GStreamer errors)
- **Scope**: All source types, especially playbin3 and urisourcebin
- **Risk**: Low (defensive programming, no functional changes)
- **Compatibility**: Full backward compatibility maintained

## Prevention

This type of issue can be prevented in the future by:

1. Always checking `caps.GetSize() > 0` before calling `caps.GetStructureAt(index)`
2. Using defensive programming patterns when working with GStreamer caps
3. Adding comprehensive logging for debugging caps-related issues
4. Testing with various HLS streams that may produce different cap structures

## Related Code Pattern

When working with GStreamer caps in Go, always use this safe pattern:

```go
caps := pad.GetCurrentCaps()
if caps == nil {
    caps = pad.QueryCaps(nil)
}

if caps != nil && caps.GetSize() > 0 {
    structure := caps.GetStructureAt(0)
    if structure != nil {
        mediaType := structure.Name()
        // Safe to process media type
    }
    caps.Unref()
} else {
    // Handle empty or nil caps appropriately
    if caps != nil {
        logger.Warnf("Empty caps (size: %d)", caps.GetSize())
        caps.Unref()
    } else {
        logger.Warn("Could not get caps")
    }
}
```
