package runtime

import "net/http"

func NewBaseMux() *http.ServeMux {
	return NewBaseMuxWithReady()
}
