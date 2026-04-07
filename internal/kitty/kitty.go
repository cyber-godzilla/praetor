package kitty

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"
)

// Encode renders an image as a Kitty graphics protocol escape sequence.
// cols and rows specify the terminal cell dimensions for display.
// The image is PNG-encoded, base64-encoded, and chunked at 4096 bytes.
func Encode(img image.Image, cols, rows int) string {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	var result strings.Builder
	const chunkSize = 4096
	for i := 0; i < len(b64); i += chunkSize {
		end := i + chunkSize
		if end > len(b64) {
			end = len(b64)
		}
		chunk := b64[i:end]
		more := 1
		if end >= len(b64) {
			more = 0
		}
		if i == 0 {
			result.WriteString(fmt.Sprintf("\033_Ga=T,f=100,t=d,c=%d,r=%d,C=1,q=2,m=%d;%s\033\\",
				cols, rows, more, chunk))
		} else {
			result.WriteString(fmt.Sprintf("\033_Gm=%d;%s\033\\", more, chunk))
		}
	}
	return result.String()
}
