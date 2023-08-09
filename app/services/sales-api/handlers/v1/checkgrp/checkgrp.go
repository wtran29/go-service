// Package checkgrp maintains the group of handlers for health checking.
package checkgrp

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	database "github.com/wtran29/go-service/business/data/database/pgx"
	"github.com/wtran29/go-service/foundation/web"
)

// Handlers manages the set of check endpoints.
type Handlers struct {
	build string
	db    *sqlx.DB
}

// New constructs a Handlers api for the check group.
func New(build string, db *sqlx.DB) *Handlers {
	return &Handlers{
		build: build,
		db:    db,
	}
}

// Readiness checks if the database is ready and if not will return a 500 status.
// Do not respond by just returning an error because further up in the call
// stack it will interpret that as a non-trusted error.
func (h Handlers) Readiness(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	ctx, span := web.AddSpan(ctx, "v1.readiness")
	defer span.End()

	status := "ok"
	statusCode := http.StatusOK
	if err := database.StatusCheck(ctx, h.db); err != nil {
		status = "db not ready"
		statusCode = http.StatusInternalServerError
	}

	data := struct {
		Status string `json:"status"`
	}{
		Status: status,
	}

	// if err := response(w, statusCode, data); err != nil {
	// 	h.Log.Errorw("readiness", "ERROR", err)
	// }

	// h.Log.Infow("readiness", "statusCode", statusCode, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)

	return web.Respond(ctx, w, data, statusCode)
}

// Liveness returns simple status info if the service is alive. If the
// app is deployed to a Kubernetes cluster, it will also return pod, node, and
// namespace details via the Downward API. The Kubernetes environment variables
// need to be set within your Pod/Deployment manifest.
func (h Handlers) Liveness(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	ctx, span := web.AddSpan(ctx, "v1.liveness")
	defer span.End()

	host, err := os.Hostname()
	if err != nil {
		host = "unavailable"
	}

	data := struct {
		Status     string `json:"status,omitempty"`
		Build      string `json:"build,omitempty"`
		Host       string `json:"host,omitempty"`
		Name       string `json:"name,omitempty"`
		PodIP      string `json:"podIP,omitempty"`
		Node       string `json:"node,omitempty"`
		Namespace  string `json:"namespace,omitempty"`
		GOMAXPROCS string `json:"GOMAXPROCS,omitempty"`
	}{
		Status:     "up",
		Build:      h.build,
		Host:       host,
		Name:       os.Getenv("KUBERNETES_NAME"),
		PodIP:      os.Getenv("KUBERNETES_POD_IP"),
		Node:       os.Getenv("KUBERNETES_NODE_NAME"),
		Namespace:  os.Getenv("KUBERNETES_NAMESPACE"),
		GOMAXPROCS: os.Getenv("GOMAXPROCS"),
	}

	// statusCode := http.StatusOK
	// if err := response(w, statusCode, data); err != nil {
	// 	h.Log.Errorw("liveness", "ERROR", err)
	// }

	// THIS IS A FREE TIMER. WE COULD UPDATE THE METRIC GOROUTINE COUNT HERE.

	// h.Log.Infow("liveness", "statusCode", statusCode, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)
	return web.Respond(ctx, w, data, http.StatusOK)
}
