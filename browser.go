package chromium

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"sync"
)

// Browser is a wrapper that embeds rod.Browser instance
type Browser struct {
	*rod.Browser
	wg       *sync.WaitGroup
	pagePool PagePool
	launcher *launcher.Launcher
}

// CleanUp wait then wipe all resources under this browser instance.
func (b *Browser) CleanUp() {
	go b.pagePool.CleanUp()
	b.wg.Wait()
	b.MustClose()
	b.launcher.Cleanup()
}

// GetPage return a page from this Browser's page pool.
// Note that it will block until a page is available from the pool.
// It is required for a caller to put back the page to the pool via PutPage function.
func (b *Browser) GetPage() *Page {
	return <-b.pagePool
}

// PutPage puts a page back to the browser's page pool.
// Note that GetPage will be blocked until there is a page available from the pool.
// By putting a page via this function will ensure next page resource to be served from a caller of GetPage function.
func (b *Browser) PutPage(p *Page) {
	b.pagePool <- p
}

// NewBrowserWithProxy returns new browser with given pool size and proxy setup.
// Note that the pagePoolSize and proxy cannot be changed once generated.
func NewBrowserWithProxy(pagePoolSize int, proxy string) (*Browser, error) {
	l := launcher.New()
	if len(proxy) > 0 {
		l = l.Proxy(proxy)
	}
	b := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	if pagePoolSize <= 0 {
		pagePoolSize = 1
	}

	pool := make(PagePool, pagePoolSize)

	wg := &sync.WaitGroup{}
	for i := 0; i < pagePoolSize; i++ {
		pool <- NewPage(b.MustPage(), wg.Done)
	}
	wg.Add(pagePoolSize)

	return &Browser{b, wg, pool, l}, nil
}

// NewBrowser returns new browser with given pool size.
// Note that the pagePoolSize cannot be changed once generated.
func NewBrowser(pagePoolSize int) (*Browser, error) {
	return NewBrowserWithProxy(pagePoolSize, "")
}
