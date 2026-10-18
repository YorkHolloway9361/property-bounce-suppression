package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	propertybounces "property-bounce-suppression"
)

func main() {
	client, err := propertybounces.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/delivery-events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var notice propertybounces.BounceNotice
		if err := json.NewDecoder(r.Body).Decode(&notice); err != nil {
			http.Error(w, "invalid delivery event", http.StatusBadRequest)
			return
		}
		decision, err := propertybounces.HandleBounce(r.Context(), client, notice)
		if err != nil {
			var remote *propertybounces.APIError
			if errors.As(err, &remote) && remote.Status >= 400 && remote.Status < 500 {
				http.Error(w, remote.Error(), http.StatusUnprocessableEntity)
				return
			}
			http.Error(w, "delivery event could not be processed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(decision)
	})
	log.Println("maintenance reminder service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
