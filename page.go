package chromium

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
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

func (p *Page) WaitJSObject(objName string) error {
	return p.WaitJSObjectFor(objName, time.Second*5)
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
			if pe := recover(); isError(pe) {
				err, _ := pe.(error)
				eChan <- replaceAbortedError(err)
			}
			defer close(eChan)
		}()

		delay := backoff

	tryNavigate:
		wait := p.MustWaitNavigation()
		done := make(chan struct{}, 1)
		go func() { defer close(done); wait(); done <- struct{}{} }()
		p.MustNavigate(url)
		if !predicate(p) {
			delay += backoff
			time.Sleep(delay)
			goto tryNavigate
		}
	}()

	return <-eChan
}

func isError(item any) bool {
	if item == nil {
		return false
	}
	_, res := item.(error)
	return res
}

// TryInput is a conjunction of Page.WaitVisibleElement and *rod.Element's Input function.
// It will propagate any error from subsequent actions by immediately returning that non-nil error.
// It will return error as nil if the action has been successfully executed.
func (p *Page) TryInput(selector, text string) error {
	eChan := make(chan error, 1)
	go func() {
		defer func() {
			if pe := recover(); isError(pe) {
				err, _ := pe.(error)
				eChan <- replaceAbortedError(err)
			}
			close(eChan)
		}()
		element, err := p.HasElement(selector)
		if err != nil {
			eChan <- err
			return
		}
		element.MustSelectAllText().MustInput(text)
	}()
	return replaceAbortedError(<-eChan)
}

// HasElement checks if any element matching the given selector.
// If exists, will return an element with no error, or vise versa.
func (p *Page) HasElement(selector string) (*rod.Element, error) {
	found, element, err := p.Has(selector)
	if err != nil {
		return nil, err
	} else if !found {
		return nil, wrap(ElementMissing, selector)
	}
	return element, nil
}

// WaitVisibleElement is a shortcut for search and wait for element to be visible (i.e. interact-ready)
// Any failure from child action will be propagated.
// Will return an element with no error on success, otherwise will return nil with error for failing reason.
func (p *Page) WaitVisibleElement(selector string) (el *rod.Element, err error) {
	if el, err = p.HasElement(selector); err != nil {
		return nil, err
	} else if err = el.WaitVisible(); err != nil {
		return nil, wrap(WaitFailed, selector)
	}
	return el, nil
}

// ClickNavigate clicks an element that is matching the given selector as criteria.
func (p *Page) ClickNavigate(selector string, timeout time.Duration) error {
	el, err := p.WaitVisibleElement(selector)
	if err != nil {
		return err
	}

	waitFunc := p.MustWaitNavigation()
	waitDone, clickFail := make(chan struct{}, 1), make(chan error, 1)

	go func(elem *rod.Element) {
		defer close(clickFail)
		if clickErr := elem.Click(proto.InputMouseButtonLeft); clickErr != nil {
			clickFail <- wrap(ClickFailed, selector)
		}
	}(el)

	go func() {
		defer func() {
			waitDone <- struct{}{}
			close(waitDone)
		}()
		waitFunc()
	}()
	for {
		select {
		case <-waitDone:
			return nil
		case e := <-clickFail:
			if e != nil {
				return e
			}
		case <-time.After(timeout):
			return TaskTimeout
		}
	}
}

// WaitJSObjectFor enforces this page to await for specified JavaScript Object to be loaded to given page,
// for specified time duration. It will wait for the item by each depth for the name by dot delimiter.
func (p *Page) WaitJSObjectFor(objName string, until time.Duration) error {
	if len(objName) == 0 {
		return nil
	} else if until == 0 {
		return TaskTimeout
	}

	timer, errChan, doneChan := time.After(until), make(chan error, 1), make(chan struct{}, 1)

	go func() {
		defer close(doneChan)
		defer close(errChan)
		begin := time.Now()
		items := strings.Split(objName, ".")
		for i := range items { // check each depth as well as checking due on each retry attempt
			if i > 0 {
				items[i] = items[i-1] + "." + items[i] // only refer last item if not the first item
			}
			script := fmt.Sprintf(`() => typeof %+v !== 'undefined'`, items[i]) // run through console
			for {
				if time.Since(begin) > until { // in case of until, we do not send doneChan signal
					return
				}
				obj, err := p.Eval(script)
				if err != nil {
					errChan <- err
					return
				}
				if obj.Value.Bool() { // found
					time.Sleep(time.Millisecond * 100)
					break
				}
			}
		}
		doneChan <- struct{}{} // success
	}()

	// evaluate which one comes first
	for {
		select {
		case err := <-errChan:
			if err != nil {
				return err
			}
		case <-timer: // on failure
			return TaskTimeout
		case <-doneChan: // on success
			return nil
		}
	}
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
