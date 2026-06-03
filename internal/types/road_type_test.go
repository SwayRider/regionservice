package types

import "testing"

// =============================================================================
// RoadTypeFromString Tests
// =============================================================================

func TestRoadTypeFromString(t *testing.T) {
	tests := []struct {
		input string
		want  RoadType
	}{
		{"motorway", MOTORWAY},
		{"MOTORWAY", MOTORWAY},
		{"Motorway", MOTORWAY},
		{"trunk", TRUNK},
		{"TRUNK", TRUNK},
		{"Trunk", TRUNK},
		{"primary", PRIMARY},
		{"PRIMARY", PRIMARY},
		{"Primary", PRIMARY},
		{"secondary", SECONDARY},
		{"SECONDARY", SECONDARY},
		{"Secondary", SECONDARY},
		{"unknown", ""},
		{"", ""},
		{"residential", ""},
		{"tertiary", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := RoadTypeFromString(tt.input)
			if got != tt.want {
				t.Errorf("RoadTypeFromString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// RoadType.String Tests
// =============================================================================

func TestRoadType_String(t *testing.T) {
	tests := []struct {
		rt   RoadType
		want string
	}{
		{MOTORWAY, "motorway"},
		{TRUNK, "trunk"},
		{PRIMARY, "primary"},
		{SECONDARY, "secondary"},
		{RoadType(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.rt.String()
			if got != tt.want {
				t.Errorf("RoadType(%q).String() = %q, want %q", tt.rt, got, tt.want)
			}
		})
	}
}
