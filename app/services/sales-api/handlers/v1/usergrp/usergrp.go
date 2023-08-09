// Package usergrp maintains the group of handlers for user access.
package usergrp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"time"

	"github.com/wtran29/go-service/business/core/user"
	"github.com/wtran29/go-service/business/core/usersummary"
	"github.com/wtran29/go-service/business/data/transaction"
	"github.com/wtran29/go-service/business/web/auth"
	v1 "github.com/wtran29/go-service/business/web/v1"
	"github.com/wtran29/go-service/business/web/v1/paging"
	"github.com/wtran29/go-service/foundation/validate"
	"github.com/wtran29/go-service/foundation/web"

	"github.com/golang-jwt/jwt/v4"
)

// Handlers manages the set of user endpoints.
type Handlers struct {
	user    *user.Core
	summary *usersummary.Core
	auth    *auth.Auth
}

// New constructs a handlers for route access.
func New(user *user.Core, summary *usersummary.Core, auth *auth.Auth) *Handlers {
	return &Handlers{
		user:    user,
		summary: summary,
		auth:    auth,
	}
}

// executeUnderTransaction constructs a new Handlers value with the core apis
// using a store transaction that was created via middleware.
func (h *Handlers) executeUnderTransaction(ctx context.Context) (*Handlers, error) {
	if tx, ok := transaction.Get(ctx); ok {
		user, err := h.user.ExecuteUnderTransaction(tx)
		if err != nil {
			return nil, err
		}

		h = &Handlers{
			user:    user,
			summary: h.summary,
			auth:    h.auth,
		}

		return h, nil
	}

	return h, nil
}

// Create adds a new user to the system.
func (h *Handlers) Create(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var anu AppNewUser
	if err := web.Decode(r, &anu); err != nil {
		return v1.NewRequestError(err, http.StatusBadRequest)
	}

	cnu, err := toCoreNewUser(anu)
	if err != nil {
		return v1.NewRequestError(err, http.StatusBadRequest)
	}

	usr, err := h.user.Create(ctx, cnu)
	if err != nil {
		if errors.Is(err, user.ErrUniqueEmail) {
			return v1.NewRequestError(err, http.StatusConflict)
		}
		return fmt.Errorf("Create -> user.Create: user[%+v]: %w", &usr, err)
	}

	return web.Respond(ctx, w, toAppUser(usr), http.StatusCreated)
}

// Update updates a user in the system.
func (h *Handlers) Update(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	h, err := h.executeUnderTransaction(ctx)
	if err != nil {
		return err
	}
	var auu AppUpdateUser
	if err := web.Decode(r, &auu); err != nil {
		return v1.NewRequestError(err, http.StatusBadRequest)
	}

	userID := auth.GetUserID(ctx)

	usr, err := h.user.QueryByID(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrNotFound):
			return v1.NewRequestError(err, http.StatusNotFound)
		default:
			return fmt.Errorf("Update -> QueryByID :userID[%s]: %w", userID, err)
		}
	}

	cuu, err := toCoreUpdateUser(auu)
	if err != nil {
		return v1.NewRequestError(err, http.StatusBadRequest)
	}

	usr, err = h.user.Update(ctx, usr, cuu)
	if err != nil {
		return fmt.Errorf("Update -> Update :userID[%s] cuu[%+v]: %w", userID, cuu, err)
	}

	return web.Respond(ctx, w, toAppUser(usr), http.StatusOK)
}

// Delete removes a user from the system.
func (h *Handlers) Delete(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	userID := auth.GetUserID(ctx)

	h, err := h.executeUnderTransaction(ctx)
	if err != nil {
		return err
	}

	usr, err := h.user.QueryByID(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrNotFound):
			return web.Respond(ctx, w, nil, http.StatusNoContent)
		default:
			return fmt.Errorf("Delete -> QueryByID :userID[%s]: %w", userID, err)
		}
	}

	if err := h.user.Delete(ctx, usr); err != nil {
		return fmt.Errorf("Delete -> Delete :userID[%s]: %w", userID, err)
	}

	return web.Respond(ctx, w, nil, http.StatusNoContent)
}

// Query returns a list of users with paging.
func (h *Handlers) Query(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	page, err := paging.ParseRequest(r)
	if err != nil {
		return err
	}

	filter, err := parseFilter(r)
	if err != nil {
		return err
	}

	orderBy, err := parseOrder(r)
	if err != nil {
		return err
	}

	users, err := h.user.Query(ctx, filter, orderBy, page.Number, page.RowsPerPage)
	if err != nil {
		return fmt.Errorf("Query -> user.Query: %w", err)
	}

	items := make([]AppUser, len(users))
	for i, usr := range users {
		items[i] = toAppUser(usr)
	}

	total, err := h.user.Count(ctx, filter)
	if err != nil {
		return fmt.Errorf("Query -> user.Count: %w", err)
	}

	return web.Respond(ctx, w, paging.NewResponse(items, total, page.Number, page.RowsPerPage), http.StatusOK)
}

// QueryByID returns a user by its ID.
func (h *Handlers) QueryByID(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	id := auth.GetUserID(ctx)

	usr, err := h.user.QueryByID(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrNotFound):
			return v1.NewRequestError(err, http.StatusNotFound)
		default:
			return fmt.Errorf("QueryByID: id[%s]: %w", id, err)
		}
	}

	return web.Respond(ctx, w, toAppUser(usr), http.StatusOK)
}

// QuerySummary returns a list of user summary data with paging.
func (h *Handlers) QuerySummary(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	page, err := paging.ParseRequest(r)
	if err != nil {
		return err
	}

	filter, err := parseSummaryFilter(r)
	if err != nil {
		return err
	}

	orderBy, err := parseSummaryOrder(r)
	if err != nil {
		return err
	}

	smms, err := h.summary.Query(ctx, filter, orderBy, page.Number, page.RowsPerPage)
	if err != nil {
		return fmt.Errorf("QuerySummary -> summary.Query: %w", err)
	}

	items := make([]AppUserSummary, len(smms))
	for i, smm := range smms {
		items[i] = toAppUserSummary(smm)
	}

	total, err := h.summary.Count(ctx, filter)
	if err != nil {
		return fmt.Errorf("QuerySummary -> summary.Count: %w", err)
	}

	return web.Respond(ctx, w, paging.NewResponse(items, total, page.Number, page.RowsPerPage), http.StatusOK)
}

// Token provides an API token for the authenticated user.
func (h *Handlers) Token(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	kid := web.Param(r, "kid")
	if kid == "" {
		return validate.NewFieldsError("Token -> kid", errors.New("missing kid"))
	}

	email, pass, ok := r.BasicAuth()
	if !ok {
		return auth.NewAuthError("must provide email and password in Basic auth")
	}

	addr, err := mail.ParseAddress(email)
	if err != nil {
		return auth.NewAuthError("invalid email format")
	}

	usr, err := h.user.Authenticate(ctx, *addr, pass)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrNotFound):
			return v1.NewRequestError(err, http.StatusNotFound)
		case errors.Is(err, user.ErrAuthenticationFailure):
			return auth.NewAuthError(err.Error())
		default:
			return fmt.Errorf("Token -> Authenticate: %w", err)
		}
	}

	claims := auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   usr.ID.String(),
			Issuer:    "service project",
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		},
		Roles: usr.Roles,
	}

	var tkn struct {
		Token string `json:"token"`
	}
	tkn.Token, err = h.auth.GenerateToken(kid, claims)
	if err != nil {
		return fmt.Errorf("Token -> GenerateToken: %w", err)
	}

	return web.Respond(ctx, w, tkn, http.StatusOK)
}
