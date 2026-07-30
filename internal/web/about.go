package web

import (
	"net/http"

	"github.com/jo3qma/ocr-mng/internal/version"
)

func (s *Server) about(w http.ResponseWriter, r *http.Request) {
	p := s.page(r, "page.about")
	unavailable := p.L.T("about.unavailable")
	info := version.Collect(version.CollectOpts{
		Context:     r.Context(),
		Unavailable: unavailable,
		OCRBinary:   s.ocrBinary,
	})
	render(w, "about", struct {
		page
		Info version.AboutInfo
	}{page: p, Info: info})
}
