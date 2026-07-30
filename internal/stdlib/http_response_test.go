package stdlib

import (
	"net/http/httptest"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestWriteWeftResponseInvalidStatusDoesNotPanic(t *testing.T) {
	response := runtime.NewMap()
	mo := response.Obj.(*runtime.MapObj)
	mo.Keys = []string{"status", "body"}
	mo.Vals["status"] = runtime.Int(42)
	mo.Vals["body"] = runtime.Str("should not be written")

	recorder := httptest.NewRecorder()
	writeWeftResponse(recorder, response)
	if recorder.Code != 500 {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if recorder.Body.String() != "invalid HTTP response status" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}
