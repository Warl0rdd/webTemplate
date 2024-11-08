package v1

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/spf13/viper"
	"time"
	"webTemplate/cmd/app"
	"webTemplate/internal/adapters/controller/api/validator"
	"webTemplate/internal/adapters/database/postgres"
	"webTemplate/internal/domain/dto"
	"webTemplate/internal/domain/entity"
	"webTemplate/internal/domain/service"
	"webTemplate/internal/domain/utils/auth"
)

type UserService interface {
	Create(ctx context.Context, registerReq dto.UserRegister) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
}

type TokenService interface {
	GenerateAuthTokens(c context.Context, userID string) (*dto.AuthTokens, error)
	GenerateToken(ctx context.Context, userID string, expires time.Time, tokenType string) (*entity.Token, error)
}

type UserHandler struct {
	userService  UserService
	tokenService TokenService
	validator    *validator.Validator
}

func NewUserHandler(app *app.App) *UserHandler {
	userStorage := postgres.NewUserStorage(app.DB)
	tokenStorage := postgres.NewTokenStorage(app.DB)

	return &UserHandler{
		userService:  service.NewUserService(userStorage),
		tokenService: service.NewTokenService(tokenStorage),
		validator:    app.Validator,
	}
}

// register godoc
// @Summary      Register a new user
// @Description  Register a new user using his email, username and password. Returns his ID, email, username, verifiedEmail boolean variable and role
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        body body  dto.UserRegister true  "User registration body object"
// @Success      201  {object}  dto.UserRegisterResponse
// @Failure      400  {object}  dto.HTTPError
// @Failure      500  {object}  dto.HTTPError
// @Router       /user/register [post]
func (h UserHandler) register(c *fiber.Ctx) error {
	var userDTO dto.UserRegister

	if err := c.BodyParser(&userDTO); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.HTTPError{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	if errValidate := h.validator.ValidateData(userDTO); errValidate != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.HTTPError{
			Code:    fiber.StatusBadRequest,
			Message: errValidate.Error(),
		})
	}

	user, errCreate := h.userService.Create(c.Context(), userDTO)
	if errCreate != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.HTTPError{
			Code:    fiber.StatusInternalServerError,
			Message: errCreate.Error(),
		})
	}

	tokens, tokensErr := h.tokenService.GenerateAuthTokens(c.Context(), user.ID)
	if tokensErr != nil || tokens == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.HTTPError{
			Code:    fiber.StatusInternalServerError,
			Message: "failed to generate auth tokens",
		})
	}

	response := dto.UserRegisterResponse{
		User: dto.UserReturn{
			ID:            user.ID,
			Email:         user.Email,
			VerifiedEmail: user.VerifiedEmail,
			Username:      user.Username,
			Role:          user.Role,
		},
		Tokens: *tokens,
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// login godoc
// @Summary      Login to existing user account.
// @Description  Login to existing user account using his email, username and password. Returns his ID, email, username, verifiedEmail boolean variable and role
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        body body  dto.UserLogin true  "User login body object"
// @Success      200  {object}  dto.UserRegisterResponse
// @Failure      400  {object}  dto.HTTPError
// @Failure      403  {object}  dto.HTTPError
// @Failure      404  {object}  dto.HTTPError
// @Failure      500  {object}  dto.HTTPError
// @Router       /user/login [post]
func (h UserHandler) login(c *fiber.Ctx) error {
	var userDTO dto.UserLogin

	if err := c.BodyParser(&userDTO); err != nil {
    return c.Status(fiber.StatusBadRequest).JSON(dto.HTTPError{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}
  
	if errValidate := h.validator.ValidateData(userDTO); errValidate != nil {
    return c.Status(fiber.StatusBadRequest).JSON(dto.HTTPError{
			Code:    fiber.StatusBadRequest,
			Message: errValidate.Error(),
		})
	}
  
  user, errFetch := h.userService.GetByEmail(c.Context(), userDTO.Email)
	if errFetch != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.HTTPError{
			Code:    fiber.StatusNotFound,
			Message: "not found",
		})
	}

	passErr := user.ComparePassword(userDTO.Password)
	if passErr != nil {
		return c.Status(fiber.StatusForbidden).JSON(dto.HTTPError{
			Code:    fiber.StatusForbidden,
			Message: "invalid password",
		})
	}

	tokens, tokensErr := h.tokenService.GenerateAuthTokens(c.Context(), user.ID)
	if tokensErr != nil || tokens == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.HTTPError{
			Code:    fiber.StatusInternalServerError,
			Message: "failed to generate auth tokens",
		})
	}

	response := dto.UserRegisterResponse{
		User: dto.UserReturn{
			ID:            user.ID,
			Email:         user.Email,
			VerifiedEmail: user.VerifiedEmail,
			Username:      user.Username,
			Role:          user.Role,
		},
		Tokens: *tokens,
  }

	return c.Status(fiber.StatusOK).JSON(response)
}

// refreshToken godoc
// @Summary      Refresh the access token
// @Description  Get a new access token using a valid refresh token
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        body body  dto.Token true  "Access token object"
// @Success      200  {object}  dto.Token
// @Failure      400  {object}  dto.HTTPError
// @Failure      403  {object}  dto.HTTPError
// @Failure      500  {object}  dto.HTTPError
// @Router       /user/refresh [post]
func (h UserHandler) refreshToken(c *fiber.Ctx) error {
	var accessTokenDTO dto.Token

	if err := c.BodyParser(&accessTokenDTO); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.HTTPError{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		})
	}

	if errValidate := h.validator.ValidateData(accessTokenDTO); errValidate != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.HTTPError{
			Code:    fiber.StatusBadRequest,
			Message: errValidate.Error(),
		})
	}
  
  userID, errToken := auth.VerifyToken(accessTokenDTO.Token, viper.GetString("service.backend.jwt.secret"), auth.TokenTypeAccess)

	if errToken != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.HTTPError{
			Code:    fiber.StatusUnauthorized,
			Message: errToken.Error(),
		})
	}

	expTime := time.Now().UTC().Add(time.Minute * time.Duration(viper.GetInt("service.backend.jwt.access-token-expiration")))

	newAccess, errNewAccess := h.tokenService.GenerateToken(c.Context(),
		userID,
		expTime,
		auth.TokenTypeAccess)

	if errNewAccess != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.HTTPError{
			Code:    fiber.StatusInternalServerError,
			Message: errNewAccess.Error(),
		})
	}

	response := dto.Token{
		Token:   newAccess.Token,
		Expires: expTime,
  }

	return c.Status(fiber.StatusOK).JSON(response)
}

func (h UserHandler) Setup(router fiber.Router) {
	userGroup := router.Group("/user")
	userGroup.Post("/register", h.register)
	userGroup.Post("/login", h.login)
	userGroup.Post("/refresh", h.refreshToken)
}
