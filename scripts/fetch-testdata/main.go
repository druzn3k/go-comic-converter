// Command fetch-testdata generates test fixtures for the go-comic-converter
// test suite: a minimal CBR (RAR) archive and a PDF with an embedded JPEG image.
//
// Output:
//   - pkg/comic/source/testdata/sample.cbr
//   - pkg/comic/source/testdata/sample.pdf
//
// The script is idempotent: re-running it does not overwrite valid fixtures.
package main

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"

	"github.com/nwaples/rardecode/v2"
	"github.com/raff/pdfreader/pdfread"
)

const fixtureDir = "pkg/comic/source/testdata"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		if err := os.MkdirAll(fixtureDir, 0755); err != nil {
			return fmt.Errorf("creating fixture dir: %w", err)
		}
	}

	if err := ensureCBR(); err != nil {
		return fmt.Errorf("CBR fixture: %w", err)
	}
	if err := ensurePDF(); err != nil {
		return fmt.Errorf("PDF fixture: %w", err)
	}

	fmt.Println("All fixtures ready.")
	return nil
}

// ── CBR fixture ─────────────────────────────────────────────────────────────

func ensureCBR() error {
	path := filepath.Join(fixtureDir, "sample.cbr")

	if _, err := os.Stat(path); err == nil {
		valid, msg := validateCBR(path)
		if valid {
			fmt.Println("CBR fixture already exists and is valid.")
			return nil
		}
		fmt.Printf("CBR fixture exists but invalid (%s), regenerating...\n", msg)
	}

	img1 := encodeJPEG(makeTestJPEG("Page 1"))
	img2 := encodeJPEG(makeTestJPEG("Page 2"))

	data := buildRAR(img1, img2)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	if valid, msg := validateCBR(path); !valid {
		os.Remove(path)
		return fmt.Errorf("generated CBR failed validation: %s", msg)
	}

	fmt.Printf("Created CBR fixture (%d bytes): %s\n", len(data), path)
	return nil
}

func buildRAR(entries ...[]byte) []byte {
	var buf bytes.Buffer

	// RAR 1.5 marker
	buf.Write([]byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x00})

	// Archive header (HEAD_ARCHIVE = 0x73)
	writeBlockHeader(&buf, 0x73, 0x0000, []byte{0x00, 0x00})

	for i := range entries {
		filename := fmt.Sprintf("page%03d.jpg", i+1)
		writeRARFile(&buf, filename, entries[i])
	}

	// End of archive (HEAD_ENDARC = 0x7B)
	writeBlockHeader(&buf, 0x7B, 0x0000, []byte{0x00, 0x00})

	return buf.Bytes()
}

// writeBlockHeader writes a RAR 1.5 block header including CRC16.
func writeBlockHeader(buf *bytes.Buffer, htype byte, flags uint16, data []byte) {
	var hdr bytes.Buffer
	hdr.WriteByte(htype)
	hdr.Write(le16(flags))
	hdr.Write(le16(uint16(7 + len(data)))) // HEAD_SIZE = CRC(2) + TYPE(1) + FLAGS(2) + SIZE(2) + data
	hdr.Write(data)

	// CRC16 = lower 16 bits of CRC32 of bytes from HEAD_TYPE onwards
	crc := crc32.ChecksumIEEE(hdr.Bytes())
	buf.Write(le16(uint16(crc)))
	buf.Write(hdr.Bytes())
}

func writeRARFile(buf *bytes.Buffer, name string, data []byte) {
	nameB := []byte(name)
	fileCRC := crc32.ChecksumIEEE(data)

	var extra bytes.Buffer
	extra.Write(le32(uint32(len(data))))    // PACK_SIZE
	extra.Write(le32(uint32(len(data))))    // UNP_SIZE
	extra.WriteByte(0x00)                   // HOST_OS
	extra.Write(le32(fileCRC))              // FILE_CRC
	extra.Write(le32(0x00000000))           // FILE_TIME
	extra.WriteByte(0x15)                   // UNP_VER = 1.5
	extra.WriteByte(0x30)                   // METHOD = STORE
	extra.Write(le16(uint16(len(nameB))))   // NAME_SIZE
	extra.Write(le32(0x00000000))           // FILE_ATTR
	extra.Write(nameB)                      // FILE_NAME

	writeBlockHeader(buf, 0x74, 0x8000, extra.Bytes()) // blockHasData flag
	buf.Write(data)
}

func validateCBR(path string) (bool, string) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Sprintf("Open: %v", err)
	}
	defer f.Close()

	r, err := rardecode.NewReader(f)
	if err != nil {
		return false, fmt.Sprintf("NewReader: %v", err)
	}

	count := 0
	for {
		if _, err := r.Next(); err != nil {
			if err == io.EOF {
				break
			}
			return false, fmt.Sprintf("Next: %v", err)
		}
		count++
	}

	if count < 2 {
		return false, fmt.Sprintf("only %d entries (need >=2)", count)
	}
	return true, "ok"
}

// ── PDF fixture ─────────────────────────────────────────────────────────────

func ensurePDF() error {
	path := filepath.Join(fixtureDir, "sample.pdf")

	if _, err := os.Stat(path); err == nil {
		valid, msg := validatePDF(path)
		if valid {
			fmt.Println("PDF fixture already exists and is valid.")
			return nil
		}
		fmt.Printf("PDF fixture exists but invalid (%s), regenerating...\n", msg)
	}

	data, err := buildPDF()
	if err != nil {
		return fmt.Errorf("building PDF: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	if valid, msg := validatePDF(path); !valid {
		os.Remove(path)
		return fmt.Errorf("generated PDF failed validation: %s", msg)
	}

	fmt.Printf("Created PDF fixture (%d bytes): %s\n", len(data), path)
	return nil
}

func buildPDF() ([]byte, error) {
	img := makeTestJPEG("PDF Page 1")
	jpgData := encodeJPEG(img)

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")

	o1 := pdf.Len()
	pdf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	o2 := pdf.Len()
	pdf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	o3 := pdf.Len()
	pdf.WriteString("3 0 obj\n")
	pdf.WriteString("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n")
	pdf.WriteString("   /Resources << /XObject << /Im0 4 0 R >> >>\n")
	pdf.WriteString("   /Contents 5 0 R\n>>\nendobj\n")

	o4 := pdf.Len()
	fmt.Fprintf(&pdf, "4 0 obj\n")
	fmt.Fprintf(&pdf, "<< /Type /XObject /Subtype /Image /Width 100 /Height 100\n")
	fmt.Fprintf(&pdf, "   /ColorSpace /DeviceGray /BitsPerComponent 8\n")
	fmt.Fprintf(&pdf, "   /Filter /DCTDecode /Length %d >>\n", len(jpgData))
	pdf.WriteString("stream\n")
	pdf.Write(jpgData)
	pdf.WriteString("\nendstream\nendobj\n")

	content := []byte("q 100 0 0 100 0 0 cm /Im0 Do Q\n")
	o5 := pdf.Len()
	pdf.WriteString("5 0 obj\n")
	fmt.Fprintf(&pdf, "<< /Length %d >>\n", len(content))
	pdf.WriteString("stream\n")
	pdf.Write(content)
	pdf.WriteString("\nendstream\nendobj\n")

	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 6\n%010d 65535 f \n", 0)
	fmt.Fprintf(&pdf, "%010d 00000 n \n", o1)
	fmt.Fprintf(&pdf, "%010d 00000 n \n", o2)
	fmt.Fprintf(&pdf, "%010d 00000 n \n", o3)
	fmt.Fprintf(&pdf, "%010d 00000 n \n", o4)
	fmt.Fprintf(&pdf, "%010d 00000 n \n", o5)
	pdf.WriteString("trailer\n<< /Size 6 /Root 1 0 R >>\n")
	fmt.Fprintf(&pdf, "startxref\n%d\n%%%%EOF\n", xref)

	return pdf.Bytes(), nil
}

func validatePDF(path string) (bool, string) {
	pdf := pdfread.Load(path)
	if pdf == nil {
		return false, "Load returned nil"
	}
	defer pdf.Close()

	pages := pdf.Pages()
	if len(pages) == 0 {
		return false, "no pages"
	}
	return true, "ok"
}

// ── helpers ─────────────────────────────────────────────────────────────────

func makeTestJPEG(_ string) image.Image {
	img := image.NewGray(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			v := uint8((x + y) * 255 / 200)
			img.Set(x, y, color.Gray{v})
		}
	}
	for y := range 10 {
		for x := 10; x < 90; x++ {
			img.Set(x, y+40, color.Gray{255})
		}
	}
	return img
}

func encodeJPEG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func le16(v uint16) []byte {
	return []byte{byte(v), byte(v >> 8)}
}

func le32(v uint32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}
