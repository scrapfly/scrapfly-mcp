package schemas

import (
	"regexp"
	"testing"
)

func TestTheCountryPatternTakesCodesListsExclusionsAndWeightsAndNothingElse(t *testing.T) {
	pattern := regexp.MustCompile(MakeCountrySchema().Pattern)

	for _, tc := range []struct {
		country string
		takes   bool
	}{
		{"", true},
		{"us", true},
		{"US", true},
		{"us,ca", true},
		{"-us", true},
		{"-us,-ca", true},
		{"us,-ca,de:5", true},
		{"us:1,ca:127", true},
		{"us:+2,ca:01", true},
		{"usa", false},
		{"u", false},
		{"-", false},
		{"-usa", false},
		{"us,", false},
		{",us", false},
		{"us,,ca", false},
		{"us:", false},
		{"us:0", false},
		{"us:128", false},
		{"us:1:2", false},
		{"us ca", false},
		{"us, ca", false},
	} {
		t.Run(tc.country, func(t *testing.T) {
			if pattern.MatchString(tc.country) != tc.takes {
				t.Errorf("the pattern takes %q: %v, want %v", tc.country, !tc.takes, tc.takes)
			}
		})
	}
}
