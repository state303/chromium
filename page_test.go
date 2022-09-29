package chromium

import (
	"context"
	"github.com/state303/chromium/internal/test/testfile"
	"github.com/state303/chromium/internal/test/testserver"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// requestCountMustBeAsExpected does assertion for server request size
func requestCountMustBeAsExpected(t *testing.T, server *testserver.TestServer, expected int) {
	got := len(server.Requests())
	assert.Equal(t, expected, got, "expected server has %+v requests, but found %+v", expected, got)
}

func Test_CleanUp_Is_Idempotent(t *testing.T) {
	_, p, _ := setupParallel(t, make([]byte, 0))
	for i := 0; i < 10; i++ {
		assert.NotPanics(t, p.CleanUp)
	}
}

func Test_HasElement_Returns_Err_When_Context_Canceled(t *testing.T) {
	_, p, _ := setupParallel(t, []byte(""))
	p.CleanUp()
	el, err := p.HasElement("any")

	assert.Nil(t, el, "expected no element from cancelled page")
	if assert.Error(t, err, "expected error when context canceled") {
		assert.ErrorContains(t, err, "context canceled")
	}
}

func Test_HasElement_Returns_Err_When_Selector_Not_Matched(t *testing.T) {
	_, p, s := setupParallel(t, []byte(""))
	p.MustNavigate(s.URL)
	selector := "li > a"
	el, err := p.HasElement(selector)
	assert.Nil(t, el, "expected no element when there is no such element")
	assert.Error(t, err, "expected error when selector has no matching element")
	assert.ErrorContains(t, err, "failed")
	assert.ErrorContains(t, err, selector)

}

func Test_TryNavigate_Waits_With_Given_Backoff(t *testing.T) {
	items := makeItems(testfile.BlankHTML, testfile.ItemsHTML, 5)
	_, p, s := setupParallel(t, items...)
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
	// prepare
	t.Parallel()
	b := PrepareBrowser(t, 5)
	t.Cleanup(b.CleanUp)
	p := b.GetPage()
	defer b.PutPage(p)
	server := testserver.WithRotatingResponses(t, testfile.ItemsHTML)
	t.Cleanup(server.Close)
	go p.CleanUp()
	err := p.TryNavigate(server.URL, func(p *Page) bool { return false }, time.Millisecond)
	assert.ErrorIs(t, err, context.Canceled)
}

func Test_TryInput_Returns_Err_When_No_Element_Found(t *testing.T) {
	t.Parallel()
	b := PrepareBrowser(t, 5)
	t.Cleanup(b.CleanUp)
	p := b.GetPage()
	defer b.PutPage(p)
	server := testserver.WithRotatingResponses(t, testfile.BlankHTML)
	t.Cleanup(server.Close)

	sel := "li > a"
	p.MustNavigate(server.URL)
	err := p.TryInput(sel, "test input")
	if assert.Error(t, err, "expected error when selector has no match") {
		assert.ErrorContains(t, err, sel, "expected error contains selector")
	}
}

func Test_TryInput_Returns_Err_When_Page_Already_Closed(t *testing.T) {
	_, p, _ := setupParallel(t, testfile.BlankHTML)
	sel := "li > a"
	p.CleanUp()
	err := p.TryInput(sel, "test")
	assert.Error(t, err, "expected error when context closed")
	assert.ErrorContains(t, err, context.Canceled.Error(), "expected error contains context canceled")
}

func Test_TryInput_Appends_Already_Inserted_Item(t *testing.T) {
	b := PrepareBrowser(t, 1)
	t.Cleanup(b.CleanUp)
	p := b.GetPage()
	defer b.PutPage(p)

	server := testserver.WithRotatingResponses(t, testfile.InputTestHTML)
	t.Cleanup(server.Close)

	assert.NoError(t, p.TryNavigate(server.URL, func(p *Page) bool { return true }, time.Second))
	requestCountMustBeAsExpected(t, server, 1)

	sel, expectedText := "#item0", "hello world"

	// initial
	assert.NoError(t, p.TryInput(sel, expectedText))
	assert.Equal(t, expectedText, p.MustElement(sel).MustText())

	// second
	assert.NoError(t, p.TryInput(sel, expectedText))
	assert.Equal(t, expectedText+expectedText, p.MustElement(sel).MustText())
}
