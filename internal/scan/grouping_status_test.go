package scan

import (
	"context"
	"testing"
)

// A scan with nothing to read still rebuilds the albums, which can take long:
// the status must say so instead of looking stuck on the walk.
func TestStatusTellsWhenBuildingTheAlbums(t *testing.T) {
	l, _ := scanned(t)
	var during Status
	l.sc.beforeGroup = func() { during = l.sc.Status() }
	if err := l.sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !during.Running || !during.Grouping {
		t.Fatalf("while grouping: %+v", during)
	}
	if st := l.sc.Status(); st.Grouping {
		t.Fatalf("after the scan: %+v", st)
	}
}
