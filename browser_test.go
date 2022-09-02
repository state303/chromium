package chromium

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"testing"
	"time"
)

func TestBrowser_CleanUp_MustBeIdempotent(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(1)
	assert.NoError(t, err)
	assert.NotPanics(t, b.CleanUp)
	assert.NotPanics(t, b.CleanUp)
}

func TestNewBrowser(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(1)
	assert.NoError(t, err)
	assert.NotNil(t, b)
	t.Cleanup(b.CleanUp)
}

func TestNewBrowser_WithEmptyProxy(t *testing.T) {
	t.Parallel()
	b, err := NewBrowserWithProxy(1, "")
	assert.NoError(t, err)
	assert.NotNil(t, b)
	t.Cleanup(b.CleanUp)
}

func TestNewBrowser_WithProxy(t *testing.T) {
	t.Parallel()
	b, err := NewBrowserWithProxy(1, "192.168.1.1:5000")
	assert.NoError(t, err)
	assert.NotNil(t, b)
	t.Cleanup(b.CleanUp)
}

func TestNewBrowser_MustHandleNegativePoolSize(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(-10)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)
	assert.Equal(t, cap(b.pagePool), 1)
}

func TestNewBrowser_MustHandleZeroPoolSize(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(0)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)
	assert.Equal(t, cap(b.pagePool), 1)
}

func TestNewBrowser_MustThrottle_WhenGetPage(t *testing.T) {
	t.Parallel()
	max, concurrency := 0, 0
	b, err := NewBrowser(5)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)

	g := new(errgroup.Group)

	for i := 0; i < 100; i++ {
		g.Go(func() error {
			p := b.GetPage()
			defer func() {
				time.Sleep(time.Millisecond * 30)
				b.PutPage(p)
				concurrency--
			}()
			concurrency++
			if concurrency > max {
				max = concurrency
			}
			if concurrency > 6 {
				return errors.New("concurrency exceeds the pool size")
			}
			return nil
		})
	}
	assert.NoError(t, g.Wait())
	assert.LessOrEqual(t, max, cap(b.pagePool)+1)
}
