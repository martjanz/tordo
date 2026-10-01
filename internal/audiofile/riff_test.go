package audiofile

import (
	"bytes"
	"errors"
	"testing"
)

func TestWAV(t *testing.T) {
	file := wavFile()
	info, err := readWAV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "wav" || info.Codec != "pcm" || !info.Lossless || info.BitDepth != 16 || info.Channels != 2 || info.SampleRate != 44100 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 1000 || info.Bitrate != 1411 {
		t.Errorf("DurationMs = %d, Bitrate = %d", info.DurationMs, info.Bitrate)
	}
	dataAt := int64(bytes.Index(file, []byte("data")) + 8)
	if info.Payload != (Range{dataAt, 176400}) {
		t.Errorf("Payload = %+v, want {%d 176400}", info.Payload, dataAt)
	}
	if info.Tags.Title != "Candombe" || info.Tags.Artist != "Rubén Rada" {
		t.Errorf("tags = %+v", info.Tags)
	}
}

func TestWAVDataSizeLargerThanFile(t *testing.T) {
	file := wavFile()
	file = file[:len(file)-76400] // the data chunk still claims 176400 bytes
	info, err := readWAV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Payload.Length != 100000 {
		t.Errorf("Payload.Length = %d, want 100000", info.Payload.Length)
	}
}

func TestAIFF(t *testing.T) {
	info, err := readAIFF(src(aiffFile()))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "aiff" || info.Codec != "pcm" || !info.Lossless || info.SampleRate != 44100 || info.DurationMs != 1000 || info.BitDepth != 16 {
		t.Errorf("tech = %+v", info)
	}
	if info.Payload.Length != 176400 {
		t.Errorf("Payload = %+v", info.Payload)
	}
	if info.Tags.Title != "Tango del este" {
		t.Errorf("Title = %q", info.Tags.Title)
	}
}

func TestExt80(t *testing.T) {
	if got := ext80(ext44100); got != 44100 {
		t.Fatalf("ext80 = %v", got)
	}
}

func TestWAVNotRIFF(t *testing.T) {
	if _, err := readWAV(src([]byte("FORM....AIFFCOMM........"))); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}
