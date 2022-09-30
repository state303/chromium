package chromium

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
	"github.com/state303/chromium/internal/test/testfile"
	"github.com/state303/chromium/internal/test/testserver"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

// requestCountMustBeAsExpected does assertion for server request size
func requestCountMustBeAsExpected(t *testing.T, server *testserver.TestServer, expected int) {
	got := len(server.Requests())
	assert.Equal(t, expected, got, "expected server has %+v requests, but found %+v", expected, got)
}

func Test_CleanUp_Is_Idempotent(t *testing.T) {
	_, p, _ := setup(t, make([]byte, 0))
	for i := 0; i < 10; i++ {
		assert.NotPanics(t, p.CleanUp)
	}
}

func Test_HasElement_Returns_Err_When_Context_Canceled(t *testing.T) {
	_, p, _ := setup(t, []byte(""))
	p.CleanUp()
	el, err := p.HasElement("any")

	assert.Nil(t, el, "expected no element from cancelled page")
	if assert.Error(t, err, "expected error when context canceled") {
		assert.ErrorContains(t, err, "context canceled")
	}
}

func Test_HasElement_Returns_Err_When_Selector_Not_Matched(t *testing.T) {
	_, p, s := setup(t, []byte(""))
	p.MustNavigate(s.URL)
	selector := "li > a"
	el, err := p.HasElement(selector)
	assert.Nil(t, el, "expected no element when there is no such element")
	assert.Error(t, err, "expected error when selector has no matching element")
	assert.ErrorContains(t, err, "failed")
	assert.ErrorContains(t, err, selector)
}

func Test_replaceAbortErr_Replaces_To_Context_Cancel(t *testing.T) {
	err := errors.New(abortedError)
	err = replaceAbortErr(err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, err.Error(), abortedError)
}

func Test_TryNavigate_Waits_With_Given_Backoff(t *testing.T) {
	items := makeItems(testfile.BlankHTML, testfile.ItemsHTML, 5)
	_, p, s := setup(t, items...)
	pred := func(p *Page) bool { return p.MustHas("li") }
	backoff := time.Millisecond * 3

	assert.NoError(t, p.TryNavigate(s.URL, pred, backoff))
	requestCountMustBeAsExpected(t, s, 6)
	requests := s.Requests()
	expected := backoff

	for i := 1; i < len(requests); i++ {
		got := requests[i].GetTime().Sub(requests[i-1].GetTime()) // calculated interval
		errFmt := "expected navigation to wait at least %+v. got: %+v"
		assert.GreaterOrEqual(t, got, expected, errFmt, expected.Milliseconds(), got.Milliseconds())
		expected += backoff
	}
}

func Test_TryNavigate_Returns_Error_When_Context_Is_Canceled(t *testing.T) {
	_, p, server := setup(t, testfile.ItemsHTML)
	go p.CleanUp()
	err := p.TryNavigate(server.URL, func(p *Page) bool { return false }, time.Millisecond)
	assert.ErrorIs(t, err, context.Canceled)
}

func Test_TryNavigate_Returns_Error_When_Cancel_During_Navigate(t *testing.T) {
	_, p, s := setup(t, testfile.BlankHTML)
	go func() {
		time.Sleep(time.Millisecond * 50)
		p.CleanUp()
	}()
	err := p.TryNavigate(s.URL, func(p *Page) bool { return false }, time.Millisecond*20)
	assert.ErrorContains(t, err, context.Canceled.Error())
}

func Test_TryInput_Returns_Err_When_No_Element_Found(t *testing.T) {
	_, p, server := setup(t, testfile.BlankHTML)
	sel := "li > a"
	p.MustNavigate(server.URL)
	err := p.TryInput(sel, "test input")
	if assert.Error(t, err, "expected error when selector has no match") {
		assert.ErrorContains(t, err, sel, "expected error contains selector")
	}
}

func Test_TryInput_Returns_Err_When_Page_Already_Closed(t *testing.T) {
	_, p, _ := setup(t, testfile.BlankHTML)
	sel := "li > a"
	p.CleanUp()
	err := p.TryInput(sel, "test")
	assert.Error(t, err, "expected error when context closed")
	assert.ErrorContains(t, err, context.Canceled.Error(), "expected error contains context canceled")
}

func Test_TryInput_Returns_Err_When_Page_Input_Failed(t *testing.T) {
	_, p, s := setup(t, testfile.InputTestHTML)
	p.MustNavigate(s.URL)
	sel := "#item0"
	go func() { time.Sleep(time.Millisecond * 10); p.CleanUp() }()
	err := p.TryInput(sel, "test item")
	assert.Error(t, err)
}

func Test_TryInput_Overwrites_Already_Inserted_Item(t *testing.T) {
	_, p, s := setup(t, testfile.InputTestHTML)
	assert.NoError(t, p.TryNavigate(s.URL, func(p *Page) bool { return true }, time.Second))
	requestCountMustBeAsExpected(t, s, 1)

	sel, expectedText := "#item0", "hello world"
	assert.NoError(t, p.TryInput(sel, expectedText))
	assert.NoError(t, p.TryInput(sel, expectedText))
	assert.Equal(t, expectedText, p.MustElement(sel).MustText())
}

func Test_Dialogs_Must_Contain_Previous_Alert(t *testing.T) {
	_, p, s := setup(t, testfile.AlertHTML)
	p.MustNavigate(s.URL)
	btn := p.MustElement("button")
	wait, handle := p.HandleDialog()
	go btn.Click(proto.InputMouseButtonLeft)
	e := wait()
	p.SaveDialog(e)
	assert.NoError(t, handle(&proto.PageHandleJavaScriptDialog{Accept: true}))

	dialogs := p.Dialogs()
	if assert.Len(t, dialogs, 1, "expected exactly 1 dialog") {
		dialog := p.dialogs[0]
		assert.NotNil(t, dialog, "expected dialog not to be nil")
		assert.Contains(t, dialog.Message, "test", "expected dialog to preserve message")
	}
}

func Test_GetVisibleElement_Returns_Err_When_No_Element_Found(t *testing.T) {
	_, p, s := setup(t, testfile.BlankHTML)
	p.MustNavigate(s.URL)
	sel := "a > li"
	el, err := p.GetVisibleElement(sel)
	assert.Nil(t, el, "expected no element when element is missing")
	assert.Error(t, err, "expected error when selector does not match")
	assert.ErrorContains(t, err, sel)
	assert.ErrorContains(t, err, "fail")
}

func Test_GetVisibleElement_Returns_Err_When_Context_Cancel(t *testing.T) {
	_, p, s := setup(t, testfile.BlankHTML)
	p.MustNavigate(s.URL)
	p.MustElement("body").MustEval("() => this.setAttribute('hidden', 'true')")
	go func() { time.Sleep(time.Millisecond * 50); p.CleanUp() }()
	el, err := p.GetVisibleElement("body")
	assert.Nil(t, el)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "body")
}

func Test_GetVisibleElement_Waits_Element_Visible(t *testing.T) {
	_, p, s := setup(t, testfile.BlankHTML)
	p.MustNavigate(s.URL)
	body := p.MustElement("body")
	body.MustEval("() => this.setAttribute('hidden', 'true')")

	delay := time.Millisecond * 100
	time.AfterFunc(delay, func() { body.MustEval("() => this.removeAttribute('hidden', 'false')") })

	begin := time.Now()
	el, err := p.GetVisibleElement("body")

	assert.GreaterOrEqual(t, time.Since(begin), delay, "expected wait time to be at least %+v", delay)
	assert.NoError(t, err)
	assert.NotNil(t, el)
}

func Test_ClickNavigate_Returns_Err_When_Fail_Wait_Visible(t *testing.T) {
	_, p, _ := setup(t)
	p.CleanUp()
	err := p.ClickNavigate("a", time.Second*5)
	assert.ErrorIs(t, err, context.Canceled)
}

func Test_ClickNavigate_Returns_Err_When_Fail_Wait_Navigate(t *testing.T) {
	delay := time.Second
	// prepare servers
	_, p, s1 := setup(t, testfile.ClickNavigateHTML)
	s2 := testserver.NewServer(func(rs []*testserver.HttpRequest, w http.ResponseWriter, r *http.Request) {
		time.AfterFunc(delay, func() { _, _ = w.Write(testfile.BlankHTML) })
	})
	t.Cleanup(s2.Close)

	// prepare attributes for click and redirect
	js := fmt.Sprintf("() => this.setAttribute('href','%+v')", s2.URL)
	p.MustNavigate(s1.URL).MustElement("a").MustEval(js)

	time.AfterFunc(time.Millisecond*100, p.CleanUp)
	err := p.ClickNavigate("a", time.Second)

	assert.Equal(t, 1, len(s1.Requests()))
	assert.Equal(t, 1, len(s2.Requests()))
	assert.Error(t, err, context.Canceled)
}

func Test_ClickNavigate_Returns_Err_When_Timeout(t *testing.T) {
	_, p, s := setup(t, testfile.ItemsHTML)
	p.MustNavigate(s.URL)
	err := p.ClickNavigate("li", time.Millisecond*10)
	assert.ErrorContains(t, err, "timeout")
}

func Test_ClickNavigate_Waits_Until_Navigate(t *testing.T) {
	delay := time.Millisecond * 80
	_, p, s1 := setup(t, testfile.ClickNavigateHTML)
	s2 := testserver.NewServer(func(rs []*testserver.HttpRequest, w http.ResponseWriter, r *http.Request) {
		time.AfterFunc(delay, func() { _, _ = w.Write(testfile.ItemsHTML) })
	})
	t.Cleanup(s2.Close)
	js := fmt.Sprintf("() => this.setAttribute('href','%+v')", s2.URL)

	p.MustNavigate(s1.URL).MustElement("a").MustEval(js)
	prevBody := p.MustHTML()
	begin := time.Now()
	err := p.ClickNavigate("a", time.Second)

	assert.GreaterOrEqual(t, time.Since(begin), delay, "expected minimum wait delay for navigation")
	assert.NoError(t, err)
	assert.NotEqual(t, prevBody, p.MustHTML())
}

func Test_WaitJSObjectFor_Returns_Err_When_Context_Canceled(t *testing.T) {
	_, p, _ := setup(t, testfile.BlankHTML)
	p.CleanUp()
	err := p.WaitJSObjectFor("test", time.Second)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func Test_WaitJSObjectFor_Returns_Err_When_Timeout(t *testing.T) {
	_, p, s := setup(t, testfile.BlankHTML)
	p.MustNavigate(s.URL)
	err := p.WaitJSObjectFor("test", time.Millisecond*50)
	assert.ErrorIs(t, err, timeout)
	err = p.WaitJSObjectFor("test", time.Duration(0))
	assert.ErrorIs(t, err, timeout)
}

func Test_WaitJSObjectFor_Returns_No_Err_When_ObjName_Is_Empty(t *testing.T) {
	_, p, _ := setup(t, testfile.BlankHTML)
	assert.NoError(t, p.WaitJSObjectFor("", 0))
}

func Test_WaitJSObjectFor_Waits_Until_Given_Object_Tree_Is_Defined(t *testing.T) {
	_, p, _ := setup(t, testfile.BlankHTML)
	objName := "first.second.third"

	time.AfterFunc(time.Millisecond*50, func() { p.MustEval("() => first = {}") })
	time.AfterFunc(time.Millisecond*300, func() { p.MustEval("() => first.second = {}") })
	time.AfterFunc(time.Millisecond*500, func() { p.MustEval("() => first.second.third = {}") })

	begin := time.Now()
	assert.NoError(t, p.WaitJSObjectFor(objName, time.Second))
	assert.Greater(t, time.Since(begin), time.Millisecond*500)
}
