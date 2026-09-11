package providers

import (
	"reflect"
	"testing"
)

func testProviderWorkspace(t *testing.T, provider any) string {
	t.Helper()

	v := reflect.ValueOf(provider)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		t.Fatalf("provider = %T, want non-nil pointer", provider)
	}

	field := v.Elem().FieldByName("workspace")
	if !field.IsValid() || field.Kind() != reflect.String {
		t.Fatalf("provider %T does not expose workspace field", provider)
	}

	return field.String()
}
