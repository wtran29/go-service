// Package user provides an example of a core business API. Right now these
// calls are just wrapping the data/data layer. But at some point you will
// want auditing or something that isn't specific to the data/store layer.
package user

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"time"

	"service/business/data/order"
	"service/business/sys/validate"

	"github.com/google/uuid"

	"golang.org/x/crypto/bcrypt"
)

// Set of error variables for CRUD operations.
var (
	ErrNotFound              = errors.New("user not found")
	ErrInvalidEmail          = errors.New("email is not valid")
	ErrUniqueEmail           = errors.New("email is not unique")
	ErrAuthenticationFailure = errors.New("authentication failed")
	ErrInvalidOrder          = errors.New("validating order by")
)

// Storer interface declares the behavior this package needs to perists and
// retrieve data.
type Storer interface {
	WithinTran(ctx context.Context, fn func(s Storer) error) error
	Create(ctx context.Context, usr User) error
	Update(ctx context.Context, usr User) error
	Delete(ctx context.Context, usr User) error
	Query(ctx context.Context, filter QueryFilter, orderBy order.By, pageNumber int, rowsPerPage int) ([]User, error)
	QueryByID(ctx context.Context, userID uuid.UUID) (User, error)
	QueryByEmail(ctx context.Context, email mail.Address) (User, error)
}

// Core manages the set of API's for user access.
type Core struct {
	storer Storer
}

// NewCore constructs a core for user api access.
func NewCore(storer Storer) *Core {
	return &Core{
		storer: storer,
	}
}

// Create inserts a new user into the database.
func (c *Core) Create(ctx context.Context, nu NewUser) (User, error) {
	if err := validate.Check(nu); err != nil {
		return User{}, fmt.Errorf("validating data: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(nu.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("generating password hash: %w", err)
	}

	now := time.Now()

	usr := User{
		ID:           uuid.New(),
		Name:         nu.Name,
		Email:        nu.Email,
		PasswordHash: hash,
		Roles:        nu.Roles,
		Enabled:      true,
		DateCreated:  now,
		DateUpdated:  now,
	}

	// This provides an example of how to execute a transaction if required.
	tran := func(s Storer) error {
		if err := s.Create(ctx, usr); err != nil {
			return fmt.Errorf("create: %w", err)
		}
		return nil
	}

	if err := c.storer.WithinTran(ctx, tran); err != nil {
		return User{}, fmt.Errorf("tran: %w", err)
	}

	// if err := c.store.Create(ctx, dbUsr); err != nil {
	// 	return User{}, fmt.Errorf("create: %w", err)
	// }

	return usr, nil
}

// Update replaces a user document in the database.
func (c *Core) Update(ctx context.Context, usr User, uu UpdateUser) (User, error) {
	if err := validate.Check(uu); err != nil {
		return User{}, fmt.Errorf("validating data: %w", err)
	}

	if uu.Name != nil {
		usr.Name = *uu.Name
	}
	if uu.Email != nil {
		usr.Email = *uu.Email
	}
	if uu.Roles != nil {
		usr.Roles = uu.Roles
	}
	if uu.Password != nil {
		pw, err := bcrypt.GenerateFromPassword([]byte(*uu.Password), bcrypt.DefaultCost)
		if err != nil {
			return User{}, fmt.Errorf("generating password hash: %w", err)
		}
		usr.PasswordHash = pw
	}
	if uu.Enabled != nil {
		usr.Enabled = *uu.Enabled
	}
	usr.DateUpdated = time.Now()

	if err := c.storer.Update(ctx, usr); err != nil {
		return User{}, fmt.Errorf("udpate: %w", err)
	}

	return usr, nil
}

// Delete removes a user from the database.
func (c *Core) Delete(ctx context.Context, usr User) error {
	if err := c.storer.Delete(ctx, usr); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

// Query retrieves a list of existing users from the database.
func (c *Core) Query(ctx context.Context, filter QueryFilter, orderBy order.By, pageNumber int, rowsPerPage int) ([]User, error) {
	if err := validate.Check(filter); err != nil {
		return nil, fmt.Errorf("validating filter: %w", err)
	}

	if err := ordering.Check(orderBy); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidOrder, err.Error())
	}

	users, err := c.storer.Query(ctx, filter, orderBy, pageNumber, rowsPerPage)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	return users, nil
}

// QueryByID gets the specified user from the database.
func (c *Core) QueryByID(ctx context.Context, userID uuid.UUID) (User, error) {
	user, err := c.storer.QueryByID(ctx, userID)
	if err != nil {
		return User{}, fmt.Errorf("query: %w", err)
	}

	return user, nil
}

// QueryByEmail gets the specified user from the database by email.
func (c *Core) QueryByEmail(ctx context.Context, email mail.Address) (User, error) {
	user, err := c.storer.QueryByEmail(ctx, email)
	if err != nil {
		return User{}, fmt.Errorf("query: %w", err)
	}

	return user, nil
}

// Authenticate finds a user by their email and verifies their password. On
// success it returns a Claims User representing this user. The claims can be
// used to generate a token for future authentication.
func (c *Core) Authenticate(ctx context.Context, email mail.Address, password string) (User, error) {
	usr, err := c.storer.QueryByEmail(ctx, email)
	if err != nil {
		return User{}, fmt.Errorf("query: %w", err)
	}

	// Compare the provided password with the saved hash. Use the bcrypt
	// comparison function so it is cryptographically secure.
	if err := bcrypt.CompareHashAndPassword(usr.PasswordHash, []byte(password)); err != nil {
		return User{}, ErrAuthenticationFailure
	}

	// If we are this far the request is valid. Create some claims for the user
	// and generate their token.
	// claims := auth.Claims{
	// 	RegisteredClaims: jwt.RegisteredClaims{
	// 		Issuer:    "service project",
	// 		Subject:   dbUsr.ID,
	// 		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
	// 		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
	// 	},
	// 	Roles: dbUsr.Roles,
	// }

	return usr, nil
}
