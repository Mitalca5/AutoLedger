package services

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 0; x < 40; x++ {
		for y := 0; y < 20; y++ {
			img.Set(x, y, color.RGBA{R: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func bookData(t *testing.T) ServiceBookData {
	vin := "VF1TEST"
	odo := 42000.0
	doc := "doc-1"
	fx := 0.5
	return ServiceBookData{
		Vehicle: &models.Vehicle{Name: "Zoé", Make: "Renault", Model: "Zoé", Vin: &vin, CurrentOdometer: 45000, Currency: "EUR"},
		Entries: []models.MaintenanceExpense{
			{ID: "2", Category: "REPAIR", Amount: 5000, Currency: "EUR", Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Description: "Plaquettes é"},
			{ID: "1", Category: "MAINTENANCE", Amount: 12050, Currency: "EUR", Date: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Odometer: &odo, DocumentID: &doc},
			{ID: "3", Category: "OTHER", Amount: 1000, Currency: "USD", FxRate: &fx, Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		},
		Attachments: map[string]ServiceBookAttachment{"doc-1": {Filename: "facture.png", MimeType: "image/png", Data: testPNG(t)}},
		Options:     ServiceBookOptions{Lang: "fr", Unit: "km"},
		Now:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestServiceBookTotalConvertsForeignAmounts(t *testing.T) {
	if got := bookData(t).total(); got != money.Cents(12050+5000+500) {
		t.Fatalf("total = %v", got)
	}
}

func TestRenderServiceBookProducesPDF(t *testing.T) {
	out, err := RenderServiceBook(bookData(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("not a PDF: %q", out[:8])
	}
	if !bytes.Contains(out, []byte("/Type /Page")) {
		t.Fatal("no page")
	}
}

func TestRenderServiceBookAnnexAddsAPageAndTolueratesBadFiles(t *testing.T) {
	d := bookData(t)
	base, _ := RenderServiceBook(ServiceBookData{Vehicle: d.Vehicle, Entries: d.Entries, Options: d.Options, Now: d.Now})
	withAnnex, err := RenderServiceBook(d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(withAnnex), "/Type /Page\n") <= strings.Count(string(base), "/Type /Page\n") {
		t.Fatal("annex page missing")
	}
	d.Attachments["doc-1"] = ServiceBookAttachment{Filename: "notes.txt", MimeType: "text/plain", Data: []byte("x")}
	if _, err := RenderServiceBook(d); err != nil {
		t.Fatalf("non-image proof must not fail: %v", err)
	}
	d.Attachments["doc-1"] = ServiceBookAttachment{Filename: "broken.png", MimeType: "image/png", Data: []byte("not a png")}
	if _, err := RenderServiceBook(d); err != nil {
		t.Fatalf("corrupt picture must not fail: %v", err)
	}
}

func TestRenderServiceBookEmpty(t *testing.T) {
	d := bookData(t)
	d.Entries = nil
	out, err := RenderServiceBook(d)
	if err != nil || !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("empty book: %v", err)
	}
}
