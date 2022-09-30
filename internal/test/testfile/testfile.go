package testfile

import "os"

const (
	base     = "testdata"
	testHTML = base + "/html"
)

var (
	BlankHTML         = readFile(testHTML + "/blank.html")
	ItemsHTML         = readFile(testHTML + "/items.html")
	InputTestHTML     = readFile(testHTML + "/input-test.html")
	AlertHTML         = readFile(testHTML + "/alert.html")
	ClickNavigateHTML = readFile(testHTML + "/click-navigate.html")
)

func readFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic("no such file: " + path)
	}
	return data
}
