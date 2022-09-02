package chromium

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPage_TryNavigate_MustWaitContentLoad(t *testing.T) {
	b, err := NewBrowser(5)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)

	g := new(errgroup.Group)

	p := b.GetPage()
	defer b.PutPage(p)

	g.Go(func() error {
		return p.TryNavigate("https://m.cgv.co.kr/")
	})
	assert.NoError(t, g.Wait())

	el, err := p.Element("body")
	assert.NoError(t, err)
	body, err := el.Text()
	assert.NoError(t, err)
	assert.NotEmpty(t, body)
}

func TestPage_TryNavigate_MustHandlePanic(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(5)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)

	p := b.GetPage()
	defer b.PutPage(p)

	assert.NoError(t, b.Close())
	assert.Error(t, p.TryNavigate("abc"))
}

func TestPage_TryNavigate_MustCancel(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(5)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)

	p := b.GetPage()
	defer b.PutPage(p)

	g := new(errgroup.Group)

	go p.CleanUp()
	g.Go(func() error {
		return p.TryNavigate("https://m.cgv.co.kr")
	})

	assert.ErrorIs(t, g.Wait(), context.Canceled)
}

func TestPage_CleanUp_MustBeIdempotent(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(5)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)

	p := b.GetPage()
	defer b.PutPage(p)

	assert.NotPanics(t, p.CleanUp)
	assert.NotPanics(t, p.CleanUp)
}

type testEmptyBodyServer struct {
	requestAt []*time.Time
}

func (t *testEmptyBodyServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	now := time.Now()
	t.requestAt = append(t.requestAt, &now)
	body := ""
	if len(t.requestAt) > 3 {
		body += "test body"
	}
	writer.Write([]byte(fmt.Sprintf("<body>%s</body>", body)))
}

func TestPage_TryNavigateWithBackoff_MustRetryWithBackoff(t *testing.T) {
	t.Parallel()

	b, err := NewBrowser(5)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)

	p := b.GetPage()
	defer b.PutPage(p)

	s := &testEmptyBodyServer{make([]*time.Time, 0)}

	server := httptest.NewServer(s)
	defer server.Close()

	backoff := time.Millisecond * 200
	expectedBackoff := backoff

	assert.NoError(t, p.TryNavigateWithBackoff(server.URL, backoff))
	assert.Equal(t, 4, len(s.requestAt), "expected exactly 4 requests to be made")
	var prev *time.Time

	for _, curr := range s.requestAt {
		if prev == nil {
			prev = curr
			continue
		}
		diff := curr.Sub(*prev)
		assert.GreaterOrEqual(t, diff, expectedBackoff,
			fmt.Sprintf("expected at least %+vms interval", expectedBackoff))
		prev = curr
		expectedBackoff += backoff
	}
}
