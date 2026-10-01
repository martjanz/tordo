package audiofile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ffprobeOnce sync.Once
	ffprobePath string
)

// HasFFprobe reports whether ffprobe is on the PATH (checked once).
func HasFFprobe() bool {
	ffprobeOnce.Do(func() { ffprobePath, _ = exec.LookPath("ffprobe") })
	return ffprobePath != ""
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType   string `json:"codec_type"`
		CodecName   string `json:"codec_name"`
		SampleRate  string `json:"sample_rate"`
		Channels    int    `json:"channels"`
		BitsPerRaw  string `json:"bits_per_raw_sample"`
		BitsPerSamp int    `json:"bits_per_sample"`
		Disposition struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
		Tags map[string]string `json:"tags"`
	} `json:"streams"`
	Format struct {
		FormatName string            `json:"format_name"`
		Duration   string            `json:"duration"`
		BitRate    string            `json:"bit_rate"`
		Tags       map[string]string `json:"tags"`
	} `json:"format"`
}

var losslessCodecs = map[string]bool{"flac": true, "alac": true, "ape": true, "wavpack": true, "tta": true, "wmalossless": true, "mlp": true, "truehd": true}

// readFFprobe asks ffprobe. Its payload is the whole file: without knowing
// the container, tags cannot be told apart from audio.
func readFFprobe(ctx context.Context, path string) (Info, error) {
	st, err := os.Stat(path)
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffprobePath, "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", path).Output()
	if err != nil {
		return Info{}, fmt.Errorf("%w: ffprobe: %v", ErrFormat, err)
	}
	var o ffprobeOutput
	if err := json.Unmarshal(out, &o); err != nil {
		return Info{}, fmt.Errorf("%w: ffprobe: %v", ErrFormat, err)
	}
	info := Info{Reader: "ffprobe", Format: strings.Split(o.Format.FormatName, ",")[0], Payload: Range{0, st.Size()}}
	for _, s := range o.Streams {
		if s.CodecType == "video" && s.Disposition.AttachedPic == 1 {
			info.Tags.HasCover = true
			continue
		}
		if s.CodecType != "audio" || info.Codec != "" {
			continue
		}
		info.Codec, info.Channels = s.CodecName, s.Channels
		info.SampleRate, _ = strconv.Atoi(s.SampleRate)
		info.Lossless = losslessCodecs[s.CodecName] || strings.HasPrefix(s.CodecName, "pcm_")
		if info.Lossless {
			info.BitDepth, _ = strconv.Atoi(s.BitsPerRaw)
			if info.BitDepth == 0 {
				info.BitDepth = s.BitsPerSamp
			}
		}
		for k, v := range s.Tags {
			info.Tags.set(k, v)
		}
	}
	if info.Codec == "" {
		return Info{}, fmt.Errorf("%w: ffprobe no encontró audio", ErrFormat)
	}
	for k, v := range o.Format.Tags {
		info.Tags.set(k, v)
	}
	if d, err := strconv.ParseFloat(o.Format.Duration, 64); err == nil {
		info.DurationMs = int64(d * 1000)
	}
	if br, err := strconv.Atoi(o.Format.BitRate); err == nil {
		info.Bitrate = br / 1000
	}
	return info, nil
}
