// Package testgrp contains all the test handlers.
package testgrp

import (
	"context"
	"math/rand"
	"net/http"

	"github.com/wtran29/go-service/foundation/web"
	"go.uber.org/zap"
)

// Handlers manages the set of check endpoints.
type Handlers struct {
	Log *zap.SugaredLogger
}

// Test handler is for development.
func Test(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if n := rand.Intn(100); n%2 == 0 {
		// test shutdown error
		// return errors.New("UNTRUSTED ERROR")
		// test trusted error
		// return v1.NewRequestError(errors.New("TRUSTED ERROR"), http.StatusBadRequest)
		panic("OHHH NOOO PANIC")

	}

	status := struct {
		Status string
	}{
		Status: "OK",
	}

	return web.Respond(ctx, w, status, http.StatusOK)
}
