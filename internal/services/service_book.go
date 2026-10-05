package services

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // registers the decoders image.DecodeConfig needs for the annex
	_ "image/png"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/servertext"
)

// fixedCostCategories are the recurring costs kept out of the service book: they are not work done on the car.
var fixedCostCategories = map[string]bool{"INSURANCE": true, "SUBSCRIPTION": true, "TAX": true, "FINANCING": true}

// ServiceBookAttachment is a proof file loaded for the annex.
type ServiceBookAttachment struct {
	Filename string
	MimeType string
	Data     []byte
}

// ServiceBookOptions selects what the book contains.
type ServiceBookOptions struct {
	Lang           string
	Unit           string
	From, To       *time.Time
	WithAttachment bool
}

// ServiceBookData is everything the PDF is drawn from, so the rendering needs no database.
type ServiceBookData struct {
	Vehicle     *models.Vehicle
	Entries     []models.MaintenanceExpense
	Attachments map[string]ServiceBookAttachment // keyed by document id
	Options     ServiceBookOptions
	Now         time.Time
}

// ServiceBookService builds the printable service book of a vehicle.
type ServiceBookService struct {
	repo     *database.Repository
	readFile func(storagePath string) ([]byte, error)
}

func NewServiceBookService(repo *database.Repository, readFile func(storagePath string) ([]byte, error)) *ServiceBookService {
	return &ServiceBookService{repo: repo, readFile: readFile}
}

// Build loads the vehicle's maintenance (fixed costs excluded) and, on request, its proof files, and renders the PDF.
func (s *ServiceBookService) Build(ctx context.Context, vehicle *models.Vehicle, userID string, opts ServiceBookOptions) ([]byte, error) {
	list, err := s.repo.ListMaintenanceExpenses(ctx, vehicle.ID)
	if err != nil {
		return nil, err
	}
	data := ServiceBookData{Vehicle: vehicle, Options: opts, Now: time.Now(), Attachments: map[string]ServiceBookAttachment{}}
	for _, m := range list {
		if fixedCostCategories[m.Category] || !serviceBookInPeriod(m.Date, opts) {
			continue
		}
		data.Entries = append(data.Entries, m)
	}
	if opts.WithAttachment {
		for _, m := range data.Entries {
			if m.DocumentID == nil {
				continue
			}
			if _, done := data.Attachments[*m.DocumentID]; done {
				continue
			}
			doc, err := s.repo.GetExpenseDocumentByID(ctx, *m.DocumentID, vehicle.ID, userID)
			if err != nil {
				continue
			}
			file := doc.Data
			if doc.StoragePath != nil && *doc.StoragePath != "" {
				if b, rerr := s.readFile(*doc.StoragePath); rerr == nil {
					file = b
				}
			}
			if len(file) > 0 {
				data.Attachments[doc.ID] = ServiceBookAttachment{Filename: doc.Filename, MimeType: doc.MimeType, Data: file}
			}
		}
	}
	return RenderServiceBook(data)
}

func serviceBookInPeriod(t time.Time, o ServiceBookOptions) bool {
	return (o.From == nil || !t.Before(*o.From)) && (o.To == nil || !t.After(*o.To))
}

func (d ServiceBookData) text(key string, args ...any) string {
	return servertext.Text(d.Options.Lang, key, args...)
}

func (d ServiceBookData) category(c string) string {
	if _, ok := map[string]bool{"MAINTENANCE": true, "REPAIR": true, "TIRES": true, "ACCESSORY": true, "OTHER": true}[c]; !ok {
		return c
	}
	return d.text("servicebook.cat." + c)
}

func (d ServiceBookData) date(t time.Time) string {
	if d.Options.Lang == "fr" {
		return t.Format("02/01/2006")
	}
	return t.Format("2006-01-02")
}

// total sums the entries in the vehicle's currency; a foreign amount is converted with its own rate, as in the app.
func (d ServiceBookData) total() money.Cents {
	var sum money.Cents
	for _, m := range d.Entries {
		switch {
		case m.Currency == "" || m.Currency == d.Vehicle.Currency:
			sum += m.Amount
		case m.FxRate != nil:
			sum += m.Amount.MulRate(*m.FxRate)
		}
	}
	return sum
}

// RenderServiceBook draws the PDF: a header with the vehicle, the maintenance table oldest first with its total,
// then one annex page per proof that is a JPEG or PNG picture.
func RenderServiceBook(d ServiceBookData) ([]byte, error) {
	entries := make([]models.MaintenanceExpense, len(d.Entries))
	copy(entries, d.Entries)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Date.Before(entries[j].Date) })
	d.Entries = entries

	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 6, tr(d.text("servicebook.page", pdf.PageNo())), "", 0, "C", false, 0, "")
	})
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetTextColor(20, 20, 20)
	pdf.CellFormat(0, 10, tr(d.text("servicebook.title")), "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(60, 60, 60)
	v := d.Vehicle
	name := strings.TrimSpace(strings.Join([]string{v.Make, v.Model}, " "))
	if v.Name != "" && !strings.EqualFold(v.Name, name) {
		name = strings.TrimSpace(v.Name + " " + name)
	}
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("%s : %s", d.text("servicebook.vehicle"), name)), "", 1, "L", false, 0, "")
	if v.Vin != nil && *v.Vin != "" {
		pdf.CellFormat(0, 6, tr(fmt.Sprintf("%s : %s", d.text("servicebook.vin"), *v.Vin)), "", 1, "L", false, 0, "")
	}
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("%s : %s", d.text("servicebook.odometer"), servertext.Distance(d.Options.Unit, v.CurrentOdometer))), "", 1, "L", false, 0, "")
	if d.Options.From != nil || d.Options.To != nil {
		from, to := "…", "…"
		if d.Options.From != nil {
			from = d.date(*d.Options.From)
		}
		if d.Options.To != nil {
			to = d.date(*d.Options.To)
		}
		pdf.CellFormat(0, 6, tr(d.text("servicebook.period", from, to)), "", 1, "L", false, 0, "")
	}
	pdf.SetFont("Helvetica", "I", 9)
	pdf.CellFormat(0, 6, tr(d.text("servicebook.generated", d.date(d.Now))), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	if len(d.Entries) == 0 {
		pdf.SetFont("Helvetica", "", 11)
		pdf.CellFormat(0, 8, tr(d.text("servicebook.empty")), "", 1, "L", false, 0, "")
		return finishPDF(pdf)
	}

	widths := []float64{22, 26, 70, 32, 30}
	heads := []string{"servicebook.col_date", "servicebook.col_km", "servicebook.col_work", "servicebook.col_amount", "servicebook.col_doc"}
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(235, 235, 240)
	pdf.SetTextColor(20, 20, 20)
	for i, h := range heads {
		pdf.CellFormat(widths[i], 7, tr(d.text(h)), "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	annex := []string{}
	annexIndex := map[string]int{}
	for _, m := range d.Entries {
		km := ""
		if m.Odometer != nil {
			km = servertext.Distance(d.Options.Unit, *m.Odometer)
		}
		work := d.category(m.Category)
		if m.Description != "" {
			work += " - " + m.Description
		}
		proof := ""
		if m.DocumentID != nil {
			if _, ok := d.Attachments[*m.DocumentID]; ok {
				idx, seen := annexIndex[*m.DocumentID]
				if !seen {
					annex = append(annex, *m.DocumentID)
					idx = len(annex)
					annexIndex[*m.DocumentID] = idx
				}
				proof = fmt.Sprintf("#%d", idx)
			} else {
				proof = "x"
			}
		}
		cur := m.Currency
		if cur == "" {
			cur = d.Vehicle.Currency
		}
		// Row height follows the wrapped work text.
		lines := pdf.SplitLines([]byte(tr(work)), widths[2])
		h := float64(len(lines)) * 5
		if h < 6 {
			h = 6
		}
		if pdf.GetY()+h > 297-18 {
			pdf.AddPage()
		}
		x, y := pdf.GetX(), pdf.GetY()
		pdf.CellFormat(widths[0], h, tr(d.date(m.Date)), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], h, tr(km), "1", 0, "L", false, 0, "")
		pdf.Rect(x+widths[0]+widths[1], y, widths[2], h, "D")
		pdf.MultiCell(widths[2], 5, tr(work), "", "L", false)
		pdf.SetXY(x+widths[0]+widths[1]+widths[2], y)
		pdf.CellFormat(widths[3], h, tr(m.Amount.String()+" "+cur), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[4], h, proof, "1", 0, "C", false, 0, "")
		pdf.SetXY(x, y+h)
	}

	pdf.SetFont("Helvetica", "B", 10)
	pdf.Ln(2)
	pdf.CellFormat(widths[0]+widths[1]+widths[2], 8, tr(d.text("servicebook.total", d.Vehicle.Currency)), "", 0, "R", false, 0, "")
	pdf.CellFormat(widths[3], 8, tr(d.total().String()), "", 1, "R", false, 0, "")

	for i, id := range annex {
		a := d.Attachments[id]
		pdf.AddPage()
		pdf.SetFont("Helvetica", "B", 12)
		pdf.SetTextColor(20, 20, 20)
		pdf.CellFormat(0, 8, tr(d.text("servicebook.annex_page", i+1, a.Filename)), "", 1, "L", false, 0, "")
		embedServiceBookImage(pdf, tr, d, id, a)
	}
	return finishPDF(pdf)
}

// embedServiceBookImage fits a JPEG or PNG proof on the page; any other file, or a picture the PDF writer cannot read,
// gets a note instead.
func embedServiceBookImage(pdf *fpdf.Fpdf, tr func(string) string, d ServiceBookData, id string, a ServiceBookAttachment) {
	imgType := ""
	switch a.MimeType {
	case "image/jpeg":
		imgType = "JPG"
	case "image/png":
		imgType = "PNG"
	}
	note := func() {
		pdf.SetFont("Helvetica", "I", 10)
		pdf.MultiCell(0, 6, tr(d.text("servicebook.not_embedded")), "", "L", false)
	}
	if imgType == "" {
		note()
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(a.Data))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		note()
		return
	}
	pdf.RegisterImageOptionsReader(id, fpdf.ImageOptions{ImageType: imgType}, bytes.NewReader(a.Data))
	if pdf.Err() {
		pdf.ClearError()
		note()
		return
	}
	maxW, maxH := 180.0, 297.0-pdf.GetY()-22
	w, h := maxW, maxW*float64(cfg.Height)/float64(cfg.Width)
	if h > maxH {
		h = maxH
		w = h * float64(cfg.Width) / float64(cfg.Height)
	}
	pdf.ImageOptions(id, 15, pdf.GetY()+2, w, h, false, fpdf.ImageOptions{ImageType: imgType}, 0, "")
}

func finishPDF(pdf *fpdf.Fpdf) ([]byte, error) {
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
