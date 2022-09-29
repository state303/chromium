package chromium

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	util "github.com/state303/chromium/internal/test/testutil"
	"golang.org/x/sync/errgroup"
	"strings"
	"sync"
	"time"
)

type Page struct {
	*rod.Page
	done    func()
	once    *sync.Once
	dialogs []*proto.PageJavascriptDialogOpening
}

// CleanUp calls page done once and only once, signalling Browser such that the page is actually closed.
func (p *Page) CleanUp() {
	p.once.Do(p.done)
	_ = p.Close()
}

// Dialogs returns history of current page's dialogs.
func (p *Page) Dialogs() []*proto.PageJavascriptDialogOpening {
	return p.dialogs
}

// SaveDialog appends given proto.PageJavascriptDialogOpening to current page's dialog history.
func (p *Page) SaveDialog(d *proto.PageJavascriptDialogOpening) {
	p.dialogs = append(p.dialogs, d)
}

// TryNavigate is a safe-guarding method of navigation with indefinite retry.
// Need of this navigation arose when navigation is succeeded with 2XX with blank HTML response.
// Logic to determine whether the navigation succeeded or not depends on Predicate for given Page.
func (p *Page) TryNavigate(url string, predicate Predicate[*Page], backoff time.Duration) error {
	eChan := make(chan error, 1)
	go func() {
		defer func() {
			if pe := recover(); pe != nil {
				if err, ok := pe.(error); ok {
					eChan <- err
				} else {
					eChan <- fmt.Errorf("%+v", pe)
				}
			}
			defer close(eChan)
		}()

		delay := backoff

	tryNavigate:
		wait := p.MustWaitNavigation()
		g := new(errgroup.Group)
		g.Go(func() error {
			wait()
			return nil
		})

		p.MustNavigate(url)
		_ = g.Wait()

		if !predicate(p) {
			delay += backoff
			time.Sleep(delay)
			goto tryNavigate
		}
	}()

	return <-eChan
}

// TryInput is a conjunction of Page.VisibleElement and *rod.Element's Input function.
// It will propagate any error from subsequent actions by immediately returning that non-nil error.
// It will return error as nil if the action has been successfully executed.
func (p *Page) TryInput(selector, text string) error {
	element, err := p.HasElement(selector)
	if err != nil {
		return err
	}
	if err = element.Input(text); err != nil {
		return util.WrapError(err, fmt.Sprintf("failed to write input for %+v", selector))
	}
	return nil
}

// HasElement checks if any element matching the given selector.
// If exists, will return an element with no error, or vise versa.
func (p *Page) HasElement(selector string) (*rod.Element, error) {
	found, element, err := p.Has(selector)
	if err != nil {
		return nil, util.WrapError(err, fmt.Sprintf("failed to locate element %+v", selector))
	} else if !found {
		return nil, fmt.Errorf("failed to locate element %+v", selector)
	}
	return element, nil
}

// VisibleElement is a shortcut for search and wait for element to be visible (i.e. interact-ready)
// Any failure from child action will be propagated.
// Will return an element with no error on success, otherwise will return nil with error for failing reason.
func (p *Page) VisibleElement(selector string) (el *rod.Element, err error) {
	if el, err = p.HasElement(selector); err != nil {
		return nil, err
	} else if err = el.WaitVisible(); err != nil {
		return nil, util.WrapError(err, fmt.Sprintf("failed waiting element %+v to be visible", selector))
	}
	return el, nil
}

// ClickNavigateElement clicks an element that is matching the given selector as criteria.
func (p *Page) ClickNavigateElement(selector string, timeout time.Duration) error {
	el, err := p.VisibleElement(selector)
	if err != nil {
		return err
	}

	waitFunc := p.MustWaitNavigation()
	waitDone, clickFail := make(chan struct{}, 1), make(chan error, 1)

	go func(elem *rod.Element) {
		defer close(clickFail)
		if clickErr := elem.Click(proto.InputMouseButtonLeft); clickErr != nil {
			clickFail <- util.WrapError(clickErr, fmt.Sprintf("failed to click element %+v", selector))
		}
	}(el)

	go func() {
		defer func() {
			waitDone <- struct{}{}
			close(waitDone)
		}()
		waitFunc()
	}()

	timer := time.After(timeout)

	select {
	case <-waitDone:
		return nil
	case e := <-clickFail:
		return e
	case <-timer:
		return fmt.Errorf("navigation triggered by element click has been timeout")
	}
}

// WaitJSObjectFor forces the page to await for specified JavaScript Object to be loaded to given page, for specified time duration.
// It will wait for each depth for the name, separated by dot delimiter.
func (p *Page) WaitJSObjectFor(name string, timeout time.Duration) error {
	timer, done := time.After(timeout), make(chan struct{}, 1)
	go func() {
		defer close(done)
		due := time.Now().Add(timeout) // set due for timeout setting
		items := strings.Split(name, ".")
		for i := range items { // check each depth as well as checking due on each retry attempt
			if i > 0 {
				items[i] = items[i-1] + "." + items[i] // only refer last item if not the first item
			}
			js := fmt.Sprintf(`() => typeof %+v !== "undefined"`, items[i]) // run through console eval func.
			for {
				if time.Now().After(due) { // in case of timeout, we do not send done signal
					return
				}
				if !p.MustEval(js).Bool() { // found
					time.Sleep(time.Millisecond * 100)
					break
				}
			}
		}
		done <- struct{}{} // success
	}()

	// evaluate which one comes first
	select {
	case <-timer: //on failure
		return fmt.Errorf("failed to observe javascript object %+v for %+v", name, timeout)
	case <-done: // on success
		return nil
	}
}

// WaitJSObject forces the page to await for specified JavaScript Object to be loaded to given page.
// It will delegate the check for Page.WaitJSObjectFor with fixed amount of time and that wait duration can be changed anytime (but still, greater than 10 second at least.)
// If you need specific, consistent time window, use Page.WaitJSObjectFor instead.
func (p *Page) WaitJSObject(name string) error {
	return p.WaitJSObjectFor(name, time.Second*30)
}

// newPage returns a page,
func newPage(p *rod.Page, done func()) *Page {
	return &Page{
		Page:    p,
		done:    done,
		once:    &sync.Once{},
		dialogs: make([]*proto.PageJavascriptDialogOpening, 0),
	}
}
