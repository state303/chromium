package chromium

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPagePool_CleanUp(t *testing.T) {
	b, err := NewBrowser(5)
	assert.NoError(t, err)
	pool := make(PagePool, 5)
	t.Cleanup(b.CleanUp)

	count := 0
	for i := 0; i < cap(pool); i++ {
		pool <- NewPage(b.MustPage(), func() { count++ })
	}

	assert.NotPanics(t, pool.CleanUp)
	assert.Equal(t, count, cap(pool))
}

func TestPagePool_MustReflectQueuePoll(t *testing.T) {
	pool := make(PagePool, 5)
	pages := make([]*Page, 5)
	for i := 0; i < 5; i++ {
		p := &Page{}
		pages[i] = p
		pool.Put(p)
	}
	for i := 0; i < 5; i++ {
		assert.Equal(t, pages[i], pool.Get())
	}
}
