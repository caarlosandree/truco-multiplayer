package pagination

import (
	"errors"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	at := time.Date(2026, 10, 14, 12, 0, 0, 123456000, time.FixedZone("BRT", -3*3600))
	gotAt, gotID, err := Decode(Encode(at, "0199e7a4-0000-7000-8000-000000000001"))
	if err != nil || !gotAt.Equal(at) || *gotID != "0199e7a4-0000-7000-8000-000000000001" {
		t.Fatalf("at=%v id=%v err=%v", gotAt, gotID, err)
	}
}

func TestDecodeEmptyAndInvalid(t *testing.T) {
	if at, id, err := Decode(""); at != nil || id != nil || err != nil {
		t.Fatal("cursor vazio deve ser a primeira página")
	}
	for _, bad := range []string{"%%%", "c2VtLXNlcGFyYWRvcg", "bm8tdGltZXxhYmM"} {
		if _, _, err := Decode(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}
