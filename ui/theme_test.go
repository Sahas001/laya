package ui

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"testing"

	colorful "github.com/lucasb-eyer/go-colorful"
)

func TestBoostHexColorIsReadable(t *testing.T) {
	for _, in := range []string{"#000000", "#101020", "#7f0000", "#ffffff", "#336699"} {
		c, err := colorful.Hex(boostHexColor(in))
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if _, _, l := c.Hsl(); l < 0.6 {
			t.Errorf("%s boosted to lightness %.2f, want >= 0.6", in, l)
		}
	}
}

func TestPaletteFromColors(t *testing.T) {
	if _, ok := paletteFromColors(nil); ok {
		t.Error("expected no palette for no colours")
	}
	p, ok := paletteFromColors([]string{"#202020", "#c03030", "#c03535"})
	if !ok {
		t.Fatal("expected a palette")
	}
	a, _ := colorful.Hex(p.Primary)
	b, _ := colorful.Hex(p.Secondary)
	if a.DistanceLab(b) < 0.15 {
		t.Errorf("primary %s and secondary %s are too similar", p.Primary, p.Secondary)
	}
}

func TestLoadArtImageDataURL(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage()); err != nil {
		t.Fatal(err)
	}
	img, err := loadArtImage("data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()))
	if err != nil || img.Bounds().Dx() != 64 {
		t.Fatalf("img=%v err=%v", img, err)
	}
	if _, err := loadArtImage("data:text/plain,hello"); err == nil {
		t.Error("expected error for non-base64 data URL")
	}
}
