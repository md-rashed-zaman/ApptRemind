package runtime

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// ReadyCheck is a named dependency check for /readyz.
type ReadyCheck struct {
	Name  string
	Check func(context.Context) error
}

func NewBaseMuxWithReady(checks ...ReadyCheck) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if len(checks) == 0 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}

		var failures []string
		for _, check := range checks {
			if check.Check == nil {
				continue
			}
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			err := check.Check(ctx)
			cancel()
			if err != nil {
				name := check.Name
				if name == "" {
					name = "dependency"
				}
				failures = append(failures, name+": "+err.Error())
			}
		}
		if len(failures) > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(strings.Join(failures, "; ")))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}
