package handler

import (
	"encoding/json"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"github.com/mailru/easyjson"
	"net/http"
)

//easyjson:json
type BalanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

func GetBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	_ = userID

	resp := BalanceResponse{
		Current:   0.0,
		Withdrawn: 0.0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	started, written, err := easyjson.MarshalToHTTPResponseWriter(&resp, w)
	if err != nil {
		if err := json.NewEncoder(w).Encode(&resp); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
	_ = started
	_ = written

}
