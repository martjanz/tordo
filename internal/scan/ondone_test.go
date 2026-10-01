package scan

import (
	"context"
	"testing"
)

func TestOnDoneAfterEachScan(t *testing.T) {
	l := newLib(t)
	n := 0
	l.sc.OnDone = func() { n++ }
	l.write(magos+"/01.mp3", tags("La leyenda del hada y el mago", "Rata Blanca", "Magos", 1))
	l.run()
	if n != 1 {
		t.Fatalf("OnDone called %d times", n)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	l.sc.Run(ctx)
	if n != 1 {
		t.Fatal("OnDone called after a cancelled scan")
	}
}
