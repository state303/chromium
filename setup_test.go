package chromium

import (
	"github.com/state303/chromium/internal/test/testfile"
	"github.com/state303/chromium/internal/test/testserver"
	"testing"
)

// Prepares and brings a new instance of browser, or fail test if browser instantiation fails
func PrepareBrowser(t *testing.T, pagePoolSize int) *Browser {
	b, err := NewBrowser(pagePoolSize)
	if err != nil {
		t.Logf("failed to instantiate new browser: %+v", err.Error())
		t.FailNow()
	}
	return b
}

func setupParallel(t *testing.T, payload ...[]byte) (*Browser, *Page, *testserver.TestServer) {
	t.Parallel()
	b := PrepareBrowser(t, 1)
	p := b.GetPage()
	t.Cleanup(func() { b.PutPage(p); b.CleanUp() })

	var s *testserver.TestServer
	if payload == nil || len(payload) == 0 {
		payload = [][]byte{testfile.BlankHTML}
	}
	s = testserver.WithRotatingResponses(t, payload...)
	t.Cleanup(s.Close)
	return b, p, s
}

// makeItems makes slice of type T that are filled with n copies of 'before', and single 'after' item.
// Simply expect total size of slice will be n+1.
// Also given n is negative, the value will be set as 0
func makeItems[T any](before, after T, n int) []T {
	if n < 0 {
		n = 0
	}
	items := make([]T, 0)
	for i := 0; i < n; i++ {
		items = append(items, before)
	}
	return append(items, after)
}
