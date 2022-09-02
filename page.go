package chromium

import (
	"github.com/go-rod/rod"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

type Page struct {
	*rod.Page
	done func()
	once *sync.Once
}

func (p *Page) CleanUp() {
	p.once.Do(p.done)
	_ = p.Close()
}

func (p *Page) TryNavigate(url string) error {
	return p.TryNavigateWithBackoff(url, time.Millisecond*500)
}

func (p *Page) TryNavigateWithBackoff(url string, backoff time.Duration) error {
	delay := time.Duration(0)
tryNavigate:
	time.Sleep(delay)
	wait := p.MustWaitNavigation()
	g := new(errgroup.Group)
	g.Go(func() error {
		wait()
		return nil
	})
	if err := p.Navigate(url); err != nil {
		return err
	}
	_ = g.Wait()

	body := p.MustElement("body").MustText()
	if len(body) == 0 {
		delay += backoff
		goto tryNavigate
	}

	return nil
}

func NewPage(p *rod.Page, done func()) *Page {
	return &Page{
		Page: p,
		done: done,
		once: &sync.Once{},
	}
}
