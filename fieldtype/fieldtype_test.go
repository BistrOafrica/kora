package fieldtype

import (
	"errors"
	"testing"
)

func TestRegisterResolveSemanticType(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Descriptor{Name: "accounting.money", Base: "core.decimal", Renderer: "currency"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	d, ok := r.Resolve("accounting.money")
	if !ok || d.Base != "core.decimal" || d.Renderer != "currency" {
		t.Fatalf("resolve = %+v ok=%v", d, ok)
	}
}

func TestDuplicateFieldTypeRejected(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(Descriptor{Name: "accounting.money", Base: "core.decimal"})
	err := r.Register(Descriptor{Name: "accounting.money", Base: "core.decimal"})
	if !errors.Is(err, ErrDuplicateFieldType) {
		t.Fatalf("want ErrDuplicateFieldType, got %v", err)
	}
}

func TestUnknownBaseTypeRejected(t *testing.T) {
	r := NewRegistry()
	err := r.Register(Descriptor{Name: "livestock.body_condition_score", Base: "core.nonexistent"})
	if !errors.Is(err, ErrUnknownBaseType) {
		t.Fatalf("want ErrUnknownBaseType, got %v", err)
	}
}

func TestNameMustBeNamespaced(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Descriptor{Name: "money", Base: "core.decimal"}); err == nil {
		t.Fatalf("expected non-namespaced name rejection")
	}
}

func TestCoreBaseTypesPreregistered(t *testing.T) {
	r := NewRegistry()
	for _, base := range []string{"core.text", "core.decimal", "core.int", "core.float", "core.bool", "core.date"} {
		if _, ok := r.Resolve(base); !ok {
			t.Fatalf("core base type %q not pre-registered", base)
		}
	}
}

func TestValidationRunsOnResolvedType(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(Descriptor{
		Name: "livestock.body_condition_score",
		Base: "core.int",
		Validate: func(v any) error {
			n, ok := v.(int)
			if !ok || n < 1 || n > 5 {
				return errors.New("body condition score must be 1..5")
			}
			return nil
		},
	})
	d, _ := r.Resolve("livestock.body_condition_score")
	if err := d.Validate(3); err != nil {
		t.Fatalf("valid value rejected: %v", err)
	}
	if err := d.Validate(9); err == nil {
		t.Fatalf("out-of-range value accepted")
	}
}
