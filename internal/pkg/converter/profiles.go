// Package converter profiles manage supported profiles for go-comic-converter.
package converter

import (
	"fmt"
	"strings"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/utils"
)

type Profile struct {
	Code            string `json:"code"`
	Description     string `json:"description"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	PreferredFormat string `json:"preferred_format,omitempty"`
}

func (p Profile) String() string {
	return p.Code + " - " + p.Description + " - " + utils.IntToString(p.Width) + "x" + utils.IntToString(p.Height)
}

type Profiles map[string]Profile

// NewProfiles Initialize list of all supported profiles.
func NewProfiles() Profiles {
	res := make(Profiles)
	for _, r := range []Profile{
		// High Resolution for Tablet
		{"HR", "High Resolution", 2400, 3840, ""},
		{"SR", "Standard Resolution", 1200, 1920, ""},
		// Kindle
		{"K1", "Kindle 1", 600, 670, "epub"},
		{"K11", "Kindle 11", 1072, 1448, "epub"},
		{"K2", "Kindle 2", 600, 670, "epub"},
		{"K34", "Kindle Keyboard/Touch", 600, 800, "epub"},
		{"K578", "Kindle", 600, 800, "epub"},
		{"KDX", "Kindle DX/DXG", 824, 1000, "epub"},
		{"KPW", "Kindle Paperwhite 1/2", 758, 1024, "epub"},
		{"KV", "Kindle Paperwhite 3/4/Voyage/Oasis", 1072, 1448, "epub"},
		{"KPW5", "Kindle Paperwhite 5/Signature Edition", 1236, 1648, "epub"},
		{"KO", "Kindle Oasis 2/3", 1264, 1680, "epub"},
		{"KS", "Kindle Scribe", 1860, 2480, "epub"},
		// Kobo
		{"KoMT", "Kobo Mini/Touch", 600, 800, "kepub"},
		{"KoG", "Kobo Glo", 768, 1024, "kepub"},
		{"KoGHD", "Kobo Glo HD", 1072, 1448, "kepub"},
		{"KoA", "Kobo Aura", 758, 1024, "kepub"},
		{"KoAHD", "Kobo Aura HD", 1080, 1440, "kepub"},
		{"KoAH2O", "Kobo Aura H2O", 1080, 1430, "kepub"},
		{"KoAO", "Kobo Aura ONE", 1404, 1872, "kepub"},
		{"KoN", "Kobo Nia", 758, 1024, "kepub"},
		{"KoC", "Kobo Clara HD/Kobo Clara 2E", 1072, 1448, "kepub"},
		{"KoL", "Kobo Libra H2O/Kobo Libra 2", 1264, 1680, "kepub"},
		{"KoF", "Kobo Forma", 1440, 1920, "kepub"},
		{"KoS", "Kobo Sage", 1440, 1920, "kepub"},
		{"KoE", "Kobo Elipsa", 1404, 1872, "kepub"},
		// reMarkable
		{"RM1", "reMarkable 1", 1404, 1872, ""},
		{"RM2", "reMarkable 2", 1404, 1872, ""},
	} {
		res[r.Code] = r
	}
	return res
}

func (p Profiles) String() string {
	s := make([]string, 0)
	for _, v := range p {
		s = append(s, fmt.Sprintf(
			"    - %-7s - %4d x %-4d - %s",
			v.Code,
			v.Width, v.Height,
			v.Description,
		))
	}
	return strings.Join(s, "\n")
}
