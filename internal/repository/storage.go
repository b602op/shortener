package repository

var Storage = make(map[string]string)

func InsertData(url string, shortURL string) {
	Storage[shortURL] = url
}

func SelectData(shortURL string) string {
	return Storage[shortURL]
}
