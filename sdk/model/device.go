package model

import (
	"fmt"
	"strings"
)

// Device is the phone an anonymous profile presents: the registration body,
// every request's device query, and the User-Agent all describe it, so one
// device id always names one model.
type Device struct {
	// Brand is device_brand and the manufacturer ("Xiaomi"); Model is the
	// model code sent as device_type, device_model, and in the User-Agent
	// ("23127PN0CC").
	Brand string `json:"brand"`
	Model string `json:"model"`
	// OSVersion is the Android release ("14"), OSAPI its API level (34), and
	// Build the build id the User-Agent carries ("UKQ1.230804.001").
	OSVersion string `json:"os_version"`
	OSAPI     int    `json:"os_api"`
	Build     string `json:"build"`
	// Resolution is "height*width" in pixels and DPI the screen density.
	Resolution string `json:"resolution"`
	DPI        int    `json:"dpi"`
	// Pad presents a tablet (dragon_device_type=pad).
	Pad bool `json:"pad,omitempty"`
	// Channel is the distribution channel the app build reports
	// ("xiaomi_8662_64" from the Xiaomi store).
	Channel string `json:"channel"`
}

// DefaultDevice is a Xiaomi 14 on Android 14 with the Xiaomi store build.
var DefaultDevice = Device{
	Brand:      "Xiaomi",
	Model:      "23127PN0CC",
	OSVersion:  "14",
	OSAPI:      34,
	Build:      "UKQ1.230804.001",
	Resolution: "1200*2670",
	DPI:        460,
	Channel:    "xiaomi_8662_64",
}

// OrDefault fills the fields d leaves empty from DefaultDevice. Pad is kept
// as given.
func (d Device) OrDefault() Device {
	def := DefaultDevice
	if d.Brand == "" {
		d.Brand = def.Brand
	}
	if d.Model == "" {
		d.Model = def.Model
	}
	if d.OSVersion == "" {
		d.OSVersion = def.OSVersion
	}
	if d.OSAPI == 0 {
		d.OSAPI = def.OSAPI
	}
	if d.Build == "" {
		d.Build = def.Build
	}
	if d.Resolution == "" {
		d.Resolution = def.Resolution
	}
	if d.DPI == 0 {
		d.DPI = def.DPI
	}
	if d.Channel == "" {
		d.Channel = def.Channel
	}
	return d
}

// Validate rejects values that would break the User-Agent or the query: text
// fields hold no control characters, semicolons, or parentheses, and the
// numbers are positive.
func (d Device) Validate() error {
	for name, v := range map[string]string{
		"brand": d.Brand, "model": d.Model, "os_version": d.OSVersion,
		"build": d.Build, "resolution": d.Resolution, "channel": d.Channel,
	} {
		if len(v) > 64 || strings.ContainsAny(v, ";()\r\n\t") {
			return fmt.Errorf("device %s %q is not usable", name, v)
		}
	}
	if d.OSAPI < 0 || d.DPI < 0 {
		return fmt.Errorf("device os_api and dpi must be positive")
	}
	if d.Resolution != "" {
		h, w, ok := strings.Cut(d.Resolution, "*")
		if !ok || !digitsOnly(h) || !digitsOnly(w) {
			return fmt.Errorf("device resolution %q is not height*width", d.Resolution)
		}
	}
	return nil
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
