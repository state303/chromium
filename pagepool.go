package chromium

type PagePool chan *Page

func (p PagePool) CleanUp() {
	for i := 0; i < cap(p); i++ {
		page := <-p
		page.CleanUp()
	}
}

func (p PagePool) Get() *Page {
	return <-p
}

func (p PagePool) Put(page *Page) {
	p <- page
}
