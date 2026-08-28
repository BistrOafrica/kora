package doctype

import "testing"

func TestNormalizeResourceName(t *testing.T) {
	tests := []struct {
		name        string
		resource    string
		displayName string
		want        string
	}{
		{name: "from display name", displayName: "Resource Name X", want: "resource_name_x"},
		{name: "keeps explicit value", resource: "ResourceNameX", displayName: "Ignored", want: "resourcenamex"},
		{name: "collapses punctuation", displayName: "Resource / Name X", want: "resource_name_x"},
		{name: "fallback", displayName: "!!!", want: "doctype"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeResourceName(tc.resource, tc.displayName); got != tc.want {
				t.Fatalf("normalizeResourceName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateAssignsResourceName(t *testing.T) {
	dt := &DocType{Name: "Resource Name X", Module: "Test"}
	if err := dt.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if dt.ResourceName != "resource_name_x" {
		t.Fatalf("ResourceName = %q, want %q", dt.ResourceName, "resource_name_x")
	}
	if dt.TableName() != "`tabResource Name X`" {
		t.Fatalf("TableName() = %q", dt.TableName())
	}
}
