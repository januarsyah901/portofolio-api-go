// Package orgexp implements: Org experience (/api/org-experience). Owner: worker M2.
// See docs/PRD.md for the full contract.
package orgexp

import (
	"net/http"

	"github.com/januarsyah901/portofolio-api-go/internal/app"
)

// Register mounts this module's routes on mux.
func Register(mux *http.ServeMux, d app.Deps) {
	_ = mux
	_ = d
	// TODO: implement (see docs/PRD.md section 4).
}
