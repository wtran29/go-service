// Package v1 contains the full set of handler functions and routes
// supported by the v1 web api.
package v1

import (
	"net/http"

	// "service/app/services/sales-api/handlers/v1/productgrp"
	"service/app/services/sales-api/handlers/v1/usergrp"
	// "service/business/core/product"
	// "service/business/core/product/stores/productdb"
	"service/business/core/user"
	"service/business/core/user/stores/usercache"
	"service/business/core/user/stores/userdb"
	"service/business/web/auth"
	"service/business/web/v1/mid"
	"service/foundation/web"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// Config contains all the mandatory systems required by handlers.
type Config struct {
	Log  *zap.SugaredLogger
	Auth *auth.Auth
	DB   *sqlx.DB
}

// Routes binds all the version 1 routes.
func Routes(app *web.App, cfg Config) {
	const version = "v1"

	authen := mid.Authenticate(cfg.Auth)
	admin := mid.Authorize(cfg.Auth, auth.RuleAdminOnly)

	ugh := usergrp.Handlers{
		User: user.NewCore(usercache.NewStore(cfg.Log, userdb.NewStore(cfg.Log, cfg.DB))),
		Auth: cfg.Auth,
	}
	app.Handle(http.MethodGet, version, "/users/token/:kid", ugh.Token)
	app.Handle(http.MethodGet, version, "/users/:page/:rows", ugh.Query, authen, admin)
	app.Handle(http.MethodGet, version, "/users/:id", ugh.QueryByID, authen)
	app.Handle(http.MethodPost, version, "/users", ugh.Create, authen, admin)
	app.Handle(http.MethodPut, version, "/users/:id", ugh.Update, authen, admin)
	app.Handle(http.MethodDelete, version, "/users/:id", ugh.Delete, authen, admin)

}
