package mid

import (
	"context"
	"net/http"

	"github.com/wtran29/go-service/business/web/auth"
	v1 "github.com/wtran29/go-service/business/web/v1"
	"github.com/wtran29/go-service/foundation/logger"
	"github.com/wtran29/go-service/foundation/validate"
	"github.com/wtran29/go-service/foundation/web"
)

// Errors handles errors coming out of the call chain. It detects normal
// application errors which are used to respond to the client in a uniform way.
// Unexpected errors (status >= 500) are logged.
func Errors(log *logger.Logger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(handler web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

			// Run the next handler and catch any propagated error.
			if err := handler(ctx, w, r); err != nil {

				// Log the error.
				log.Error(ctx, "message", "msg", err)

				ctx, span := web.AddSpan(ctx, "business.web.v1.mid.error")
				span.RecordError(err)
				span.End()

				// Build out the error response.
				var er v1.ErrorResponse
				var status int

				switch {
				case v1.IsRequestError(err):
					reqErr := v1.GetRequestError(err)

					if validate.IsFieldErrors(reqErr.Err) {
						fieldErrors := validate.GetFieldErrors(reqErr.Err)
						er = v1.ErrorResponse{
							Error:  "data validation error",
							Fields: fieldErrors.Fields(),
						}
						status = reqErr.Status
						break
					}

					er = v1.ErrorResponse{
						Error: reqErr.Error(),
					}
					status = reqErr.Status

				case auth.IsAuthError(err):
					er = v1.ErrorResponse{
						Error: http.StatusText(http.StatusUnauthorized),
					}
					status = http.StatusUnauthorized

				default:
					er = v1.ErrorResponse{
						Error: http.StatusText(http.StatusInternalServerError),
					}
					status = http.StatusInternalServerError
				}

				// Respond with the error back to the client.
				if err := web.Respond(ctx, w, er, status); err != nil {
					return err
				}

				// If we receive the shutdown err we need to return it
				// back to the base handler to shutdown the service.
				if web.IsShutdown(err) {
					return err
				}
			}
			// The error has been handled so we can stop propagating it.
			return nil

		}
		return h
	}

	return m
}
