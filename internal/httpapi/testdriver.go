package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"
)

// withTestDriver registers injected-clock controls only when SHELFMATE_TEST_DRIVER=1.
// Production serve never sets that variable, so this is not a public endpoint.
func (s Server) withTestDriver(mux *http.ServeMux) {
	if os.Getenv("SHELFMATE_TEST_DRIVER") != "1" || s.Session == nil {
		return
	}
	var mu sync.Mutex
	mux.HandleFunc("/api/test/clock", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		var body struct {
			Now string `json:"now"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Now == "" {
			writeJSON(w, 400, map[string]string{"error": "now must be an RFC3339 instant"})
			return
		}
		when, err := time.Parse(time.RFC3339, body.Now)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "now must be an RFC3339 instant"})
			return
		}
		mu.Lock()
		frozen := when
		s.Session.SetClock(func() time.Time { return frozen })
		mu.Unlock()
		writeJSON(w, 200, map[string]any{"now": frozen.UTC()})
	})
}
