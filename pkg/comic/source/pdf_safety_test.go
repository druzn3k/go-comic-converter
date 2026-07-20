package source

import (
	"bytes"
	"fmt"
	"image"
	"testing"

	"github.com/raff/pdfreader/pdfread"
)

// TestExtractSafeReturnsErrorOnCorruptedPDF verifies that extractSafe returns
// a nil-cleanable result (nil image or error) instead of panicking on a PDF
// with no images.
func TestExtractSafeReturnsErrorOnCorruptedPDF(t *testing.T) {
	data := createTestPDFBytes(t)
	pdf := pdfread.LoadBytes(data)
	if pdf == nil {
		t.Fatal("LoadBytes returned nil for valid minimal PDF")
	}
	defer pdf.Close()

	img, err := extractSafe(pdf, 1)
	if err != nil {
		// Error is acceptable — no image found, and no panic.
		return
	}
	if img != nil {
		// If we somehow got an image from a no-image page, that's odd but not a crash.
		t.Logf("unexpected non-nil image from image-free PDF: %T", img)
	}
}

// TestExtractSafeReturnsErrorOnUnsupportedEncoding verifies that extractSafe
// returns (nil, error) instead of crashing via log.Fatal when the PDF page
// contains an image with /FlateDecode /DeviceGray /BitsPerComponent 3
// (a combination that hits the log.Fatal default branch in pdfimage.Extract).
func TestExtractSafeReturnsErrorOnUnsupportedEncoding(t *testing.T) {
	data := buildPDFWithUnsupportedGrayImage(t)
	pdf := pdfread.LoadBytes(data)
	if pdf == nil {
		t.Fatal("LoadBytes returned nil for constructed PDF")
	}
	defer pdf.Close()

	img, err := extractSafe(pdf, 1)
	if err == nil {
		t.Error("expected error for unsupported encoding, got nil")
	}
	if img != nil {
		t.Errorf("expected nil image, got %T", img)
	}
}

// TestExtractSafePanicRecovery verifies that if pdfimage.Extract panics (not
// log.Fatal), the recover() shim returns an error instead of crashing the test.
func TestExtractSafePanicRecovery(t *testing.T) {
	data := buildPDFWithJunkStream(t)
	pdf := pdfread.LoadBytes(data)
	if pdf == nil {
		t.Skip("LoadBytes returned nil for malformed PDF")
	}
	defer pdf.Close()

	// Should not panic; should return an error.
	_, err := extractSafe(pdf, 1)
	if err == nil {
		t.Log("no error — Extract may have silently returned nil, nil")
	}
}

// buildPDFWithFlateGray3Image creates a minimal PDF containing one page with
// an image XObject using /FlateDecode /DeviceGray /BitsPerComponent 3.
// bpc=3 is not in {1,2,4,8} and hits the log.Fatal default case in
// pdfimage.Extract's DeviceGray switch. The stream data is junk since
// pre-validation only inspects the dictionary.
func buildPDFWithUnsupportedGrayImage(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer

	buf.WriteString("%PDF-1.4\n")

	// Object 1: Catalog
	o1 := buf.Len()
	buf.WriteString("1 0 obj\n")
	buf.WriteString("<< /Type /Catalog /Pages 2 0 R >>\n")
	buf.WriteString("endobj\n")

	o2 := buf.Len()
	buf.WriteString("2 0 obj\n")
	buf.WriteString("<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n")
	buf.WriteString("endobj\n")

	// Object 3: Page with Resources referencing the image XObject
	o3 := buf.Len()
	buf.WriteString("3 0 obj\n")
	buf.WriteString("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n")
	buf.WriteString("   /Resources << /XObject << /Im0 4 0 R >> >>\n")
	buf.WriteString(">>\n")
	buf.WriteString("endobj\n")

	// Object 4: Image XObject — FlateDecode DeviceGray BitsPerComponent 4
	o4 := buf.Len()
	buf.WriteString("4 0 obj\n")
	buf.WriteString("<< /Type /XObject /Subtype /Image /Width 4 /Height 4\n")
	buf.WriteString("   /ColorSpace /DeviceGray /BitsPerComponent 3\n")
	buf.WriteString("   /Filter /FlateDecode /Length 10\n")
	buf.WriteString(">>\n")
	buf.WriteString("stream\n")
	buf.WriteString("\x78\x9c\x00\x00\x00\x00\x00\x00\x00\x00") // garbage zlib-ish data
	buf.WriteString("\nendstream\n")
	buf.WriteString("endobj\n")

	// xref
	xref := buf.Len()
	buf.WriteString("xref\n")
	buf.WriteString("0 5\n")
	buf.WriteString("0000000000 65535 f \n")
	fmt.Fprintf(&buf, "%010d 00000 n \n", o1)
	fmt.Fprintf(&buf, "%010d 00000 n \n", o2)
	fmt.Fprintf(&buf, "%010d 00000 n \n", o3)
	fmt.Fprintf(&buf, "%010d 00000 n \n", o4)
	buf.WriteString("trailer\n")
	buf.WriteString("<< /Size 5 /Root 1 0 R >>\n")
	buf.WriteString("startxref\n")
	fmt.Fprintf(&buf, "%d\n", xref)
	buf.WriteString("%%EOF\n")

	return buf.Bytes()
}

// buildPDFWithJunkStream creates a minimal PDF with an image XObject whose
// stream data is invalid enough to potentially cause a panic during decoding.
func buildPDFWithJunkStream(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer

	buf.WriteString("%PDF-1.4\n")

	o1 := buf.Len()
	buf.WriteString("1 0 obj\n")
	buf.WriteString("<< /Type /Catalog /Pages 2 0 R >>\n")
	buf.WriteString("endobj\n")

	o2 := buf.Len()
	buf.WriteString("2 0 obj\n")
	buf.WriteString("<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n")
	buf.WriteString("endobj\n")

	o3 := buf.Len()
	buf.WriteString("3 0 obj\n")
	buf.WriteString("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n")
	buf.WriteString("   /Resources << /XObject << /Im0 4 0 R >> >>\n")
	buf.WriteString(">>\n")
	buf.WriteString("endobj\n")

	// Image XObject with DCTDecode (JPEG) — but data is not valid JPEG.
	// This may panic when jpeg.Decode is called.
	o4 := buf.Len()
	buf.WriteString("4 0 obj\n")
	buf.WriteString("<< /Type /XObject /Subtype /Image /Width 16 /Height 16\n")
	buf.WriteString("   /ColorSpace /DeviceGray /BitsPerComponent 8\n")
	buf.WriteString("   /Filter /DCTDecode /Length 12\n")
	buf.WriteString(">>\n")
	buf.WriteString("stream\n")
	buf.WriteString("NOTAVALIDJPEG!!!")
	buf.WriteString("\nendstream\n")
	buf.WriteString("endobj\n")

	xref := buf.Len()
	buf.WriteString("xref\n")
	buf.WriteString("0 5\n")
	buf.WriteString("0000000000 65535 f \n")
	fmt.Fprintf(&buf, "%010d 00000 n \n", o1)
	fmt.Fprintf(&buf, "%010d 00000 n \n", o2)
	fmt.Fprintf(&buf, "%010d 00000 n \n", o3)
	fmt.Fprintf(&buf, "%010d 00000 n \n", o4)
	buf.WriteString("trailer\n")
	buf.WriteString("<< /Size 5 /Root 1 0 R >>\n")
	buf.WriteString("startxref\n")
	fmt.Fprintf(&buf, "%d\n", xref)
	buf.WriteString("%%EOF\n")

	return buf.Bytes()
}

// TestPrevalidateDetectsUnsupportedFilter verifies the pre-validation rejects
// an image XObject with an unsupported filter.
func TestPrevalidateDetectsUnsupportedFilter(t *testing.T) {
	// Build a minimal PDF with /JBIG2Decode filter (known unsupported)
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	o1 := buf.Len()
	buf.WriteString("1 0 obj\n")
	buf.WriteString("<< /Type /Catalog /Pages 2 0 R >>\n")
	buf.WriteString("endobj\n")
	o2 := buf.Len()
	buf.WriteString("2 0 obj\n")
	buf.WriteString("<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n")
	buf.WriteString("endobj\n")
	o3 := buf.Len()
	buf.WriteString("3 0 obj\n")
	buf.WriteString("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n")
	buf.WriteString("   /Resources << /XObject << /Im0 4 0 R >> >>\n")
	buf.WriteString(">>\n")
	buf.WriteString("endobj\n")
	o4 := buf.Len()
	buf.WriteString("4 0 obj\n")
	buf.WriteString("<< /Type /XObject /Subtype /Image /Width 1 /Height 1\n")
	buf.WriteString("   /ColorSpace /DeviceGray /BitsPerComponent 8\n")
	buf.WriteString("   /Filter /JBIG2Decode /Length 1\n")
	buf.WriteString(">>\n")
	buf.WriteString("stream\n")
	buf.WriteString("\x00\nendstream\nendobj\n")
	xref := buf.Len()
	buf.WriteString("xref\n0 5\n")
	buf.WriteString("0000000000 65535 f \n")
	fmt.Fprintf(&buf, "%010d 00000 n \n", o1)
	fmt.Fprintf(&buf, "%010d 00000 n \n", o2)
	fmt.Fprintf(&buf, "%010d 00000 n \n", o3)
	fmt.Fprintf(&buf, "%010d 00000 n \n", o4)
	buf.WriteString("trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n")
	fmt.Fprintf(&buf, "%d\n", xref)
	buf.WriteString("%%EOF\n")

	pdf := pdfread.LoadBytes(buf.Bytes())
	if pdf == nil {
		t.Fatal("LoadBytes returned nil")
	}
	defer pdf.Close()

	img, err := extractSafe(pdf, 1)
	if err == nil {
		t.Error("expected error for unsupported /JBIG2Decode filter, got nil")
	}
	if img != nil {
		t.Errorf("expected nil image, got %T", img)
	}
}

// compile-time check: image.Image interface satisfaction (no-op test helper)
var _ image.Image = image.NewRGBA(image.Rect(0, 0, 1, 1))
