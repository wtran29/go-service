// Package testgrp contains all the test handlers.
package testgrp

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// Handlers manages the set of check endpoints.
type Handlers struct {
	Log *zap.SugaredLogger
}

// Test handler is for development.
func Test(w http.ResponseWriter, r *http.Request) {
	// if n := rand.Intn(100); n%2 == 0 {
	// 	// return errors.New("untrusted error")
	// 	return webv1.NewRequestError(errors.New("trusted error"), http.StatusBadRequest)

	// }
	status := struct {
		Status string
	}{
		Status: "OK",
	}

	json.NewEncoder(w).Encode(status)

	// return web.Respond(ctx, w, status, http.StatusOK)
}
