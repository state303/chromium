package chromium

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"sync"
	"testing"
	"time"
)

func Test_Browser_CleanUp_Is_Idempotent(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(1)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)
	for i := 0; i < 10; i++ {
		assert.NotPanics(t, b.CleanUp)
	}
}

func Test_NewBrowser_Returns_No_Error(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(1)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)
	assert.NotNil(t, b)
}

func Test_NewBrowserWithProxy_Returns_No_Error_When_Proxy_Is_Empty(t *testing.T) {
	t.Parallel()
	b, err := NewBrowserWithProxy(1, "")
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)
	assert.NotNil(t, b)
}

func Test_NewBrowserWithProxy_Returns_Browser_When_Proxy_Is_Not_Empty(t *testing.T) {
	t.Parallel()
	b, err := NewBrowserWithProxy(1, "192.168.1.1:5000")
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)
	assert.NotNil(t, b)
}

func Test_NewBrowser_Sets_Pool_Size_To_One_When_Param_Is_Negative(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(-10)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)
	assert.Equal(t, cap(b.pagePool), 1)
}

func Test_NewBrowser_Sets_Pool_Size_To_One_When_Param_Is_Zero(t *testing.T) {
	t.Parallel()
	b, err := NewBrowser(0)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)
	assert.Equal(t, cap(b.pagePool), 1)
}

func Test_GetPage_Returns_When_Page_Is_Back_To_Pool(t *testing.T) {
	t.Parallel()
	max, concurrency := 0, 0
	b, err := NewBrowser(5)
	assert.NoError(t, err)
	t.Cleanup(b.CleanUp)

	g := new(errgroup.Group)

	countLock, maxCountLock := &sync.Mutex{}, &sync.Mutex{}
	addCount := func() { countLock.Lock(); defer countLock.Unlock(); concurrency++ }
	reduceCount := func() { countLock.Lock(); defer countLock.Unlock(); concurrency-- }
	getCount := func() int { countLock.Lock(); defer countLock.Unlock(); return concurrency }
	setMaxCount := func() {
		maxCountLock.Lock()
		defer maxCountLock.Unlock()
		if c := getCount(); c > max {
			max = c
		}
	}
	getMaxCount := func() int { maxCountLock.Lock(); defer maxCountLock.Unlock(); return max }

	for i := 0; i < 100; i++ {
		g.Go(func() error {
			p := b.GetPage()
			defer func() {
				time.Sleep(time.Millisecond * 30)
				b.PutPage(p)
				reduceCount()
			}()
			addCount()
			setMaxCount()
			if getMaxCount() > 5 {
				return errors.New("concurrency exceeds the pool size")
			}
			return nil
		})
	}

	assert.NoError(t, g.Wait())
	assert.LessOrEqual(t, max, cap(b.pagePool))
}
