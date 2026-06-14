package attendance

import "testing"

// TestHaversine sanity-checks the great-circle distance: zero for identical points,
// and a known ~111 km per degree of latitude at the equator.
func TestHaversine(t *testing.T) {
	if d := haversine(-6.2256, 106.7997, -6.2256, 106.7997); d != 0 {
		t.Errorf("identical points: got %d m, want 0", d)
	}
	// 1° of latitude ≈ 111.19 km (mean Earth radius 6371 km).
	d := haversine(0, 0, 1, 0)
	if d < 111_000 || d > 111_400 {
		t.Errorf("1° latitude: got %d m, want ~111.2 km", d)
	}
	// A short, realistic site-radius hop (~100 m range) is symmetric.
	a := haversine(-6.2256, 106.7997, -6.2265, 106.8003)
	b := haversine(-6.2265, 106.8003, -6.2256, 106.7997)
	if a != b {
		t.Errorf("haversine not symmetric: %d vs %d", a, b)
	}
	if a <= 0 {
		t.Errorf("nearby points: got %d m, want > 0", a)
	}
}

// TestEvalGeofence covers inside/outside and the no-coordinates skip.
func TestEvalGeofence(t *testing.T) {
	siteLat, siteLng := -6.2256, 106.7997

	// Same point, radius 100 → inside, distance 0.
	inside, dist, have := evalGeofence(siteLat, siteLng, &siteLat, &siteLng, 100)
	if !have || !inside || dist != 0 {
		t.Errorf("on-center: inside=%v dist=%d have=%v, want inside, 0, true", inside, dist, have)
	}

	// A point ~512 m away with a 100 m radius → outside.
	farLat := -6.2256 + 0.0046 // ~512 m north
	inside, dist, have = evalGeofence(farLat, siteLng, &siteLat, &siteLng, 100)
	if !have {
		t.Fatalf("far point: have=false, want true")
	}
	if inside {
		t.Errorf("far point: inside=true, want false (dist=%d, radius=100)", dist)
	}
	if dist <= 100 {
		t.Errorf("far point: dist=%d, want > 100", dist)
	}

	// No site coordinates → geofence skipped (inside=true, have=false).
	inside, dist, have = evalGeofence(siteLat, siteLng, nil, nil, 100)
	if have || !inside || dist != 0 {
		t.Errorf("no-coords: inside=%v dist=%d have=%v, want inside, 0, false", inside, dist, have)
	}
}

// TestAppendUnique verifies the flag dedupe used on clock-out.
func TestAppendUnique(t *testing.T) {
	out := appendUnique([]string{"LATE"}, "OUTSIDE_GEOFENCE")
	if len(out) != 2 {
		t.Fatalf("append new: len=%d, want 2", len(out))
	}
	out = appendUnique(out, "LATE")
	if len(out) != 2 {
		t.Errorf("append dup: len=%d, want 2 (no dup)", len(out))
	}
}

// TestItoa covers the OUT_OF_GEOFENCE field formatter.
func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 7: "7", 100: "100", 512: "512", -3: "-3"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Errorf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}
