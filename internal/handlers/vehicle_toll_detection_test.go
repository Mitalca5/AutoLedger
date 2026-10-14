package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestVehicleTollDetectionModeIsValidatedStoredAndReported(t *testing.T) {
	a := newVehicleAPI(t, "toll-mode")

	for _, body := range []string{`{"name":"x","toll_detection":"always"}`, `{"name":"x","toll_detection":""}`} {
		a.do(a.other, http.StatusBadRequest, "POST", "/", body)
	}

	created := decodeObj(t, a.do(a.other, http.StatusCreated, "POST", "/", `{"name":"Zoe"}`))
	if created["toll_detection"] != "auto" || created["toll_detection_active"] != false {
		t.Fatalf("a new vehicle should be on auto and inactive for an English owner: %v", created)
	}
	forced := decodeObj(t, a.do(a.other, http.StatusCreated, "POST", "/", `{"name":"Kangoo","toll_detection":"on"}`))
	if forced["toll_detection"] != "on" || forced["toll_detection_active"] != true {
		t.Fatalf("an explicit on should be active: %v", forced)
	}

	id := created["id"].(string)
	a.do(a.other, http.StatusBadRequest, "PUT", "/"+id, `{"name":"Zoe","toll_detection":"sometimes"}`)

	// An omitted value keeps the mode; an explicit one replaces it and is read back.
	kept := decodeObj(t, a.do(a.other, http.StatusOK, "PUT", "/"+id, `{"name":"Zoe 2"}`))
	if kept["toll_detection"] != "auto" {
		t.Fatalf("an omitted mode should be kept: %v", kept)
	}
	on := decodeObj(t, a.do(a.other, http.StatusOK, "PUT", "/"+id, `{"name":"Zoe 2","toll_detection":"on"}`))
	if on["toll_detection"] != "on" || on["toll_detection_active"] != true {
		t.Fatalf("on should be saved and active: %v", on)
	}
	var got map[string]any
	if err := json.Unmarshal(a.do(a.other, http.StatusOK, "GET", "/"+id, ""), &got); err != nil {
		t.Fatal(err)
	}
	if got["toll_detection"] != "on" || got["toll_detection_active"] != true {
		t.Fatalf("the mode should be read back: %v", got)
	}
	off := decodeObj(t, a.do(a.other, http.StatusOK, "PUT", "/"+id, `{"name":"Zoe 2","toll_detection":"off"}`))
	if off["toll_detection"] != "off" || off["toll_detection_active"] != false {
		t.Fatalf("off should be saved and inactive: %v", off)
	}
}

func TestTollDetectionRoutesAnswerConflictWhenTheDetectionIsOff(t *testing.T) {
	a := newVehicleAPI(t, "toll-gate")
	h := NewDriveHandler(a.repo, nil, nil)
	a.router.Post("/{vehicleId}/drives/{driveId}/detect-tolls", h.DetectTolls)
	a.router.Post("/{vehicleId}/drives/{driveId}/apply-toll-estimate", h.ApplyTollEstimate)
	a.router.Post("/{vehicleId}/drives/apply-toll-estimates", h.ApplyTollEstimatesBulk)

	// The fixture vehicle is on auto with an English owner: no sign of France, so the detection is off.
	for _, path := range []string{
		"/" + a.vid + "/drives/d1/detect-tolls",
		"/" + a.vid + "/drives/d1/apply-toll-estimate",
		"/" + a.vid + "/drives/apply-toll-estimates",
	} {
		body := a.do(a.ownerID, http.StatusConflict, "POST", path, `{"drive_ids":["d1"]}`)
		var out struct{ Code string }
		if err := json.Unmarshal(body, &out); err != nil || out.Code != "toll.detection_disabled" {
			t.Errorf("%s: code = %q (%v), want toll.detection_disabled (%s)", path, out.Code, err, body)
		}
	}
}
