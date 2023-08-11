// Package productgrp maintains the group of handlers for product access.
package productgrp

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/wtran29/go-service/business/core/product"
	"github.com/wtran29/go-service/business/core/user"
	"github.com/wtran29/go-service/business/data/transaction"
	v1 "github.com/wtran29/go-service/business/web/v1"
	"github.com/wtran29/go-service/business/web/v1/paging"
	"github.com/wtran29/go-service/foundation/web"

	"github.com/google/uuid"
)

// Set of error variables for handling product group errors.
var (
	ErrInvalidID = errors.New("ID is not in its proper form")
)

// Handlers manages the set of product endpoints.
type Handlers struct {
	product *product.Core
	user    *user.Core
}

// New constructs a handlers for route access.
func New(product *product.Core, user *user.Core) *Handlers {
	return &Handlers{
		product: product,
		user:    user,
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

		product, err := h.product.ExecuteUnderTransaction(tx)
		if err != nil {
			return nil, err
		}

		h = &Handlers{
			user:    user,
			product: product,
		}

		return h, nil
	}

	return h, nil
}

// Create adds a new product to the system.
func (h *Handlers) Create(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var anp AppNewProduct
	if err := web.Decode(r, &anp); err != nil {
		return v1.NewRequestError(err, http.StatusBadRequest)
	}

	np, err := toCoreNewProduct(anp)
	if err != nil {
		return v1.NewRequestError(err, http.StatusBadRequest)
	}

	prd, err := h.product.Create(ctx, np)
	if err != nil {
		return fmt.Errorf("Create -> Create: app[%+v]: %w", anp, err)
	}

	return web.Respond(ctx, w, toAppProduct(prd), http.StatusCreated)
}

// Update updates a product in the system.
func (h *Handlers) Update(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	h, err := h.executeUnderTransaction(ctx)
	if err != nil {
		return err
	}

	var aup AppUpdateProduct
	if err := web.Decode(r, &aup); err != nil {
		return fmt.Errorf("unable to decode payload: %w", err)
	}

	productID, err := uuid.Parse(web.Param(r, "product_id"))
	if err != nil {
		return v1.NewRequestError(ErrInvalidID, http.StatusBadRequest)
	}

	prd, err := h.product.QueryByID(ctx, productID)
	if err != nil {
		switch {
		case errors.Is(err, product.ErrNotFound):
			return v1.NewRequestError(err, http.StatusNotFound)
		default:
			return fmt.Errorf("Update -> QueryByID: productID[%s]: %w", productID, err)
		}
	}

	prd, err = h.product.Update(ctx, prd, toCoreUpdateProduct(aup))
	if err != nil {
		return fmt.Errorf("Update -> Update: productID[%s] app[%+v]: %w", productID, aup, err)
	}

	return web.Respond(ctx, w, toAppProduct(prd), http.StatusOK)
}

// Delete removes a product from the system.
func (h *Handlers) Delete(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	h, err := h.executeUnderTransaction(ctx)
	if err != nil {
		return err
	}
	prdID, err := uuid.Parse(web.Param(r, "product_id"))
	if err != nil {
		return v1.NewRequestError(ErrInvalidID, http.StatusBadRequest)
	}

	prd, err := h.product.QueryByID(ctx, prdID)
	if err != nil {
		switch {
		// case errors.Is(err, product.ErrInvalidID):
		// 	return v1.NewRequestError(err, http.StatusBadRequest)
		case errors.Is(err, product.ErrNotFound):

			// Don't send StatusNotFound here since the call to Delete
			// below won't if this product is not found. We only know
			// this because we are doing the Query for the UserID.
			return v1.NewRequestError(err, http.StatusNoContent)
		default:
			return fmt.Errorf("Delete -> QueryByID: productID[%s]: %w", prdID, err)
		}
	}

	if err := h.product.Delete(ctx, prd); err != nil {
		return fmt.Errorf("Delete -> Delete: productID[%s]: %w", prdID, err)
	}

	return web.Respond(ctx, w, nil, http.StatusNoContent)
}

// Query returns a list of products with paging.
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

	prds, err := h.product.Query(ctx, filter, orderBy, page.Number, page.RowsPerPage)
	if err != nil {
		return fmt.Errorf("Query -> product.Query: %w", err)
	}

	// -------------------------------------------------------------------------
	// Capture the unique set of users

	users := make(map[uuid.UUID]user.User)
	if len(prds) > 0 {
		for _, prd := range prds {
			users[prd.UserID] = user.User{}
		}

		userIDs := make([]uuid.UUID, 0, len(users))
		for userID := range users {
			userIDs = append(userIDs, userID)
		}

		usrs, err := h.user.QueryByIDs(ctx, userIDs)
		if err != nil {
			return fmt.Errorf("Query -> user.QueryByIDs: userIDs[%s]: %w", userIDs, err)
		}

		for _, usr := range usrs {
			users[usr.ID] = usr
		}
	}

	// -------------------------------------------------------------------------

	total, err := h.product.Count(ctx, filter)
	if err != nil {
		return fmt.Errorf("Query -> Count: %w", err)
	}

	return web.Respond(ctx, w, paging.NewResponse(toAppProductsDetails(prds, users), total, page.Number, page.RowsPerPage), http.StatusOK)
}

// QueryByID returns a product by its ID.
func (h *Handlers) QueryByID(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	prdID, err := uuid.Parse(web.Param(r, "product_id"))
	if err != nil {
		return v1.NewRequestError(ErrInvalidID, http.StatusBadRequest)
	}

	prod, err := h.product.QueryByID(ctx, prdID)
	if err != nil {
		switch {
		case errors.Is(err, product.ErrInvalidID):
			return v1.NewRequestError(err, http.StatusBadRequest)
		case errors.Is(err, product.ErrNotFound):
			return v1.NewRequestError(err, http.StatusNotFound)
		default:
			return fmt.Errorf("QueryByID -> QueryByID: productID[%s]: %w", prdID, err)
		}
	}

	return web.Respond(ctx, w, toAppProduct(prod), http.StatusOK)
}
