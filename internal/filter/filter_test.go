package filter

import (
	"net/url"
	"testing"

	"wallpaper-api/internal/errs"
	"wallpaper-api/internal/manifest"
)

func TestParseNormalizesTags(t *testing.T) {
	q := url.Values{"tags": {" anime , blue ", ",anime,", ""}}
	c, err := Parse(q)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []string{"anime", "blue"}
	if len(c.Tags) != len(want) {
		t.Fatalf("tags = %v, want %v", c.Tags, want)
	}
	for i := range want {
		if c.Tags[i] != want[i] {
			t.Fatalf("tags = %v, want %v", c.Tags, want)
		}
	}
}

func TestParseRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name  string
		query url.Values
		code  string
	}{
		{"orientation", url.Values{"orientation": {"diagonal"}}, "invalid_orientation"},
		{"min_width", url.Values{"min_width": {"wide"}}, "invalid_min_width"},
		{"min_height", url.Values{"min_height": {"-5"}}, "invalid_min_height"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.query)
			var apiErr *errs.Error
			if !asErr(err, &apiErr) || apiErr.Code != tc.code || apiErr.Status != 400 {
				t.Fatalf("Parse() error = %v, want code %q", err, tc.code)
			}
		})
	}
}

func TestMatchUsesIntersectionSemantics(t *testing.T) {
	img := manifest.Image{
		ID: "01", Category: "anime", Tags: []string{"anime", "blue"},
		Width: 1920, Height: 1080, Orientation: "landscape",
	}
	cases := []struct {
		name string
		c    Criteria
		want bool
	}{
		{"empty", Criteria{}, true},
		{"all tags present", Criteria{Tags: []string{"anime", "blue"}}, true},
		{"missing tag", Criteria{Tags: []string{"anime", "red"}}, false},
		{"category match", Criteria{Category: "anime"}, true},
		{"category mismatch", Criteria{Category: "landscape"}, false},
		{"orientation match", Criteria{Orientation: "landscape"}, true},
		{"orientation mismatch", Criteria{Orientation: "portrait"}, false},
		{"min width ok", Criteria{MinWidth: 1920}, true},
		{"min width too big", Criteria{MinWidth: 1921}, false},
		{"min height too big", Criteria{MinHeight: 1081}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.c.Match(img); got != tc.want {
				t.Fatalf("Match() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplyPreservesOrder(t *testing.T) {
	images := []manifest.Image{
		{ID: "01", Category: "anime"},
		{ID: "02", Category: "landscape"},
		{ID: "03", Category: "anime"},
	}
	got := Apply(images, Criteria{Category: "anime"})
	if len(got) != 2 || got[0].ID != "01" || got[1].ID != "03" {
		t.Fatalf("Apply() = %+v", got)
	}
}

func asErr(err error, target **errs.Error) bool {
	e, ok := err.(*errs.Error)
	if ok {
		*target = e
	}
	return ok
}
