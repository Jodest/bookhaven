package http

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	authDomain "github.com/bookhaven/backend/internal/modules/auth/domain"
	"github.com/bookhaven/backend/internal/modules/auth/service"
	"github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles authentication HTTP requests.
type Handler struct {
	service       *service.Service
	secureCookies bool
}

// Option configures a Handler.
type Option func(*Handler)

// WithSecureCookies sets the Secure attribute on auth cookies.
func WithSecureCookies(secure bool) Option {
	return func(h *Handler) { h.secureCookies = secure }
}

// NewHandler creates a new auth Handler.
func NewHandler(service *service.Service, opts ...Option) *Handler {
	h := &Handler{service: service}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

type registerRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type authResponse struct {
	User         userResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
}

type userResponse struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Name          *string   `json:"name"`
	AvatarURL     *string   `json:"avatar_url"`
	Role          string    `json:"role"`
	ProfileType   string    `json:"profile_type"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{
		ID:            u.ID.String(),
		Email:         u.Email,
		Name:          u.Name,
		AvatarURL:     u.AvatarURL,
		Role:          string(u.Role),
		ProfileType:   string(u.ProfileType),
		EmailVerified: u.EmailVerifiedAt != nil,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

// registerResponse is the 201 body of Register: no tokens, the user must
// verify the email and then log in.
type registerResponse struct {
	Email                string `json:"email" example:"anna@example.com"`
	VerificationRequired bool   `json:"verification_required" example:"true"`
}

type resendVerificationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

type forgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// resetPasswordRequest: the password rule is identical to registerRequest.
type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password" validate:"required,min=8"`
}

// statusResponse is the success body of the email verification endpoints.
type statusResponse struct {
	Status string `json:"status" example:"verified"`
}

// errorResponse is an error body with a machine-readable code.
type errorResponse struct {
	Error string `json:"error" example:"invalid token"`
	Code  string `json:"code" example:"invalid_token"`
}

// rateLimitedResponse is the 429 body; retry_after_seconds duplicates the
// Retry-After header, which is not readable cross-origin.
type rateLimitedResponse struct {
	Error             string `json:"error" example:"too many requests"`
	Code              string `json:"code" example:"rate_limited"`
	RetryAfterSeconds int    `json:"retry_after_seconds" example:"42"`
}

// Machine-readable error codes of the auth and email verification endpoints.
const (
	codeInvalidToken     = "invalid_token"
	codeTokenExpired     = "token_expired"
	codeTokenUsed        = "token_used"
	codeRateLimited      = "rate_limited"
	codeEmailSendFailed  = "email_send_failed"
	codeEmailNotVerified = "email_not_verified"
	codeInvalidEmail     = "invalid_email"
	codeInvalidPassword  = "invalid_password"
)

// Register godoc
// @Summary Регистрация пользователя
// @Description Создаёт пользователя (имя 2-255 символов после обрезки пробелов, пароль 8 символов - 72 байта в UTF-8) и отправляет письмо со ссылкой подтверждения email. Вход не выполняется и cookies не устанавливаются: войти можно только после подтверждения email.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body registerRequest true "Данные регистрации"
// @Success 201 {object} registerResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/register [post]
func (h *Handler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	user, err := h.service.Register(c.Request().Context(), req.Email, req.Password, req.Name)
	if err != nil {
		if errors.Is(err, service.ErrInvalidName) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "name must be between 2 and 255 characters"})
		}
		if errors.Is(err, service.ErrPasswordTooLong) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "password must be at most 72 bytes"})
		}
		if errors.Is(err, service.ErrUserExists) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "user already exists"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to register"})
	}

	// No auto-login: unverified users cannot authenticate.
	return c.JSON(http.StatusCreated, registerResponse{
		Email:                user.Email,
		VerificationRequired: true,
	})
}

// Login godoc
// @Summary Вход пользователя
// @Description Сначала проверяется пароль, затем подтверждение email. Если пароль верный, но email не подтверждён, возвращается 403 с кодом email_not_verified, токены не создаются и cookies не устанавливаются. Попытки ограничиваются: 60 попыток за 15 минут с одного IP (считаются все попытки, включая успешные) и 10 неудачных за 15 минут для одного адреса (верный пароль или сброс пароля обнуляет счётчик адреса); при превышении - 429 с кодом rate_limited, одинаковый для существующих и неизвестных адресов.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "Учётные данные"
// @Success 200 {object} authResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} errorResponse "code: email_not_verified"
// @Failure 429 {object} rateLimitedResponse "code: rate_limited"
// @Header 429 {integer} Retry-After "Секунд до следующей попытки"
// @Failure 500 {object} map[string]string
// @Router /auth/login [post]
func (h *Handler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	user, tokens, err := h.service.Login(c.Request().Context(), req.Email, req.Password, c.RealIP())
	if err != nil {
		var rateErr *service.RateLimitError
		if errors.As(err, &rateErr) {
			return writeRateLimited(c, rateErr)
		}
		if errors.Is(err, service.ErrInvalidCredentials) {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		}
		if errors.Is(err, service.ErrEmailNotVerified) {
			return c.JSON(http.StatusForbidden, errorResponse{Error: "email not verified", Code: codeEmailNotVerified})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to login"})
	}

	h.setAuthCookies(c, tokens)

	return c.JSON(http.StatusOK, authResponse{
		User:         toUserResponse(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	})
}

// Refresh godoc
// @Summary Обновить пару токенов
// @Description Принимает refresh-токен в теле запроса или в cookie refresh_token. Если email пользователя не подтверждён, все его refresh-токены отзываются, auth-cookies очищаются и возвращается 401 с кодом email_not_verified.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body refreshRequest false "Refresh-токен (опционально, можно передать через cookie)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} errorResponse "code: email_not_verified (без кода - недействительный токен)"
// @Router /auth/refresh [post]
func (h *Handler) Refresh(c echo.Context) error {
	var req refreshRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Try refresh token from cookie first
	refreshToken := req.RefreshToken
	if refreshToken == "" {
		cookie, err := c.Cookie("refresh_token")
		if err == nil && cookie != nil {
			refreshToken = cookie.Value
		}
	}

	if refreshToken == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "refresh token required"})
	}

	tokens, err := h.service.RefreshTokens(c.Request().Context(), refreshToken)
	if errors.Is(err, service.ErrEmailNotVerified) {
		h.clearAuthCookies(c)
		return c.JSON(http.StatusUnauthorized, errorResponse{Error: "email not verified", Code: codeEmailNotVerified})
	}
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired refresh token"})
	}

	h.setAuthCookies(c, tokens)

	return c.JSON(http.StatusOK, tokens)
}

// Logout godoc
// @Summary Выход пользователя
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/logout [post]
func (h *Handler) Logout(c echo.Context) error {
	userIDStr, ok := c.Get("user_id").(string)
	if !ok || userIDStr == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid user"})
	}

	if err := h.service.Logout(c.Request().Context(), userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to logout"})
	}

	// Clear cookies
	h.clearAuthCookies(c)

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// Me godoc
// @Summary Текущий пользователь
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} userResponse
// @Failure 401 {object} map[string]string
// @Router /auth/me [get]
func (h *Handler) Me(c echo.Context) error {
	user, ok := c.Get("user").(*domain.User)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	return c.JSON(http.StatusOK, toUserResponse(user))
}

// VerifyEmail godoc
// @Summary Подтвердить email
// @Description Подтверждает адрес электронной почты по одноразовому токену из письма. Не требует авторизации и не меняет cookies. Коды ошибок: invalid_token, token_expired, token_used.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body verifyEmailRequest true "Токен из ссылки в письме"
// @Success 200 {object} statusResponse "status: verified или already_verified"
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /auth/verify-email [post]
func (h *Handler) VerifyEmail(c echo.Context) error {
	var req verifyEmailRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid token", Code: codeInvalidToken})
	}

	alreadyVerified, err := h.service.VerifyEmail(c.Request().Context(), req.Token)
	switch {
	case err == nil && alreadyVerified:
		return c.JSON(http.StatusOK, statusResponse{Status: "already_verified"})
	case err == nil:
		return c.JSON(http.StatusOK, statusResponse{Status: "verified"})
	case errors.Is(err, service.ErrVerificationTokenInvalid):
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid token", Code: codeInvalidToken})
	case errors.Is(err, service.ErrVerificationTokenExpired):
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "token expired", Code: codeTokenExpired})
	case errors.Is(err, service.ErrVerificationTokenUsed):
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "token already used", Code: codeTokenUsed})
	default:
		c.Logger().Error(err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to verify email"})
	}
}

// ResendVerification godoc
// @Summary Повторно отправить письмо подтверждения email
// @Description Публичный эндпоинт. Если адрес принадлежит неподтверждённому аккаунту, удаляет его неиспользованные ссылки подтверждения и отправляет новую. Ответ 202 одинаков для любого корректного адреса (неизвестного, уже подтверждённого и т.д.), чтобы не раскрывать, какие адреса зарегистрированы. Ограничения: 20 запросов в час с одного IP; для одного адреса 1 раз в минуту и 5 раз в сутки. Коды ошибок: invalid_email, rate_limited, email_send_failed.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body resendVerificationRequest true "Email аккаунта"
// @Success 202 {object} statusResponse "status: sent"
// @Failure 400 {object} errorResponse
// @Failure 429 {object} rateLimitedResponse
// @Header 429 {integer} Retry-After "Секунд до следующей попытки"
// @Failure 500 {object} errorResponse
// @Router /auth/resend-verification [post]
func (h *Handler) ResendVerification(c echo.Context) error {
	var req resendVerificationRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid email", Code: codeInvalidEmail})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid email", Code: codeInvalidEmail})
	}

	err := h.service.ResendVerification(c.Request().Context(), req.Email, c.RealIP())
	if err == nil {
		return c.JSON(http.StatusAccepted, statusResponse{Status: "sent"})
	}
	var rateErr *service.RateLimitError
	if errors.As(err, &rateErr) {
		return writeRateLimited(c, rateErr)
	}
	c.Logger().Error(err)
	return c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to send verification email", Code: codeEmailSendFailed})
}

// ForgotPassword godoc
// @Summary Запросить сброс пароля
// @Description Публичный эндпоинт. Если адрес принадлежит аккаунту с паролем (подтверждённому или нет), удаляет его неиспользованные ссылки сброса и отправляет письмо с новой ссылкой (действует 1 час, одноразовая). Ответ 202 одинаков для любого корректного адреса (неизвестного, без пароля и т.д.), чтобы не раскрывать, какие адреса зарегистрированы. Ограничения: 20 запросов в час с одного IP; для одного адреса 1 раз в минуту и 5 раз в сутки. Коды ошибок: invalid_email, rate_limited, email_send_failed.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body forgotPasswordRequest true "Email аккаунта"
// @Success 202 {object} statusResponse "status: sent"
// @Failure 400 {object} errorResponse
// @Failure 429 {object} rateLimitedResponse
// @Header 429 {integer} Retry-After "Секунд до следующей попытки"
// @Failure 500 {object} errorResponse
// @Router /auth/forgot-password [post]
func (h *Handler) ForgotPassword(c echo.Context) error {
	var req forgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid email", Code: codeInvalidEmail})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid email", Code: codeInvalidEmail})
	}

	err := h.service.ForgotPassword(c.Request().Context(), req.Email, c.RealIP())
	if err == nil {
		return c.JSON(http.StatusAccepted, statusResponse{Status: "sent"})
	}
	var rateErr *service.RateLimitError
	if errors.As(err, &rateErr) {
		return writeRateLimited(c, rateErr)
	}
	c.Logger().Error(err)
	return c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to send password reset email", Code: codeEmailSendFailed})
}

// ResetPassword godoc
// @Summary Сбросить пароль
// @Description Публичный эндпоинт. Устанавливает новый пароль по одноразовому токену из письма. Пароль проверяется до токена (8 символов - 72 байта в UTF-8), поэтому отклонённый пароль не расходует ссылку. При успехе: токен помечается использованным, email считается подтверждённым, все refresh-токены пользователя отзываются, auth-cookies очищаются. Вход не выполняется. Коды ошибок: invalid_password, invalid_token, token_used, token_expired.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body resetPasswordRequest true "Токен из ссылки в письме и новый пароль"
// @Success 200 {object} statusResponse "status: password_reset"
// @Failure 400 {object} errorResponse
// @Failure 500 {object} map[string]string
// @Router /auth/reset-password [post]
func (h *Handler) ResetPassword(c echo.Context) error {
	var req resetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid token", Code: codeInvalidToken})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error(), Code: codeInvalidPassword})
	}

	err := h.service.ResetPassword(c.Request().Context(), req.Token, req.Password)
	switch {
	case err == nil:
		h.clearAuthCookies(c)
		return c.JSON(http.StatusOK, statusResponse{Status: "password_reset"})
	case errors.Is(err, service.ErrPasswordTooLong):
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "password must be at most 72 bytes", Code: codeInvalidPassword})
	case errors.Is(err, service.ErrResetTokenInvalid):
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid token", Code: codeInvalidToken})
	case errors.Is(err, service.ErrResetTokenExpired):
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "token expired", Code: codeTokenExpired})
	case errors.Is(err, service.ErrResetTokenUsed):
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "token already used", Code: codeTokenUsed})
	default:
		c.Logger().Error(err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to reset password"})
	}
}

// writeRateLimited writes the 429 response (Retry-After header and body,
// at least 1 second) for a rate-limited request.
func writeRateLimited(c echo.Context, rateErr *service.RateLimitError) error {
	seconds := int(math.Ceil(rateErr.RetryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	c.Response().Header().Set("Retry-After", strconv.Itoa(seconds))
	return c.JSON(http.StatusTooManyRequests, rateLimitedResponse{
		Error:             "too many requests",
		Code:              codeRateLimited,
		RetryAfterSeconds: seconds,
	})
}

// authCookie builds an auth cookie with the single source of attributes
// (HttpOnly, SameSite=Strict, Secure from config). Set and clear both go
// through it so their attributes always match.
func (h *Handler) authCookie(name, value, path string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		Path:     path,
		MaxAge:   maxAge,
	}
}

const (
	accessTokenPath  = "/"
	refreshTokenPath = "/api/v1/auth/refresh"
	hasSessionPath   = "/"
	// sessionMaxAge is shared by refresh_token and has_session so they can
	// never diverge in the browser.
	sessionMaxAge = 60 * 60 * 24 * 30 // 30 days
)

func (h *Handler) setAuthCookies(c echo.Context, tokens *authDomain.TokenPair) {
	if tokens == nil {
		return
	}
	c.SetCookie(h.authCookie("access_token", tokens.AccessToken, accessTokenPath, int(tokens.ExpiresIn)))
	c.SetCookie(h.authCookie("refresh_token", tokens.RefreshToken, refreshTokenPath, sessionMaxAge))
	// has_session is a non-sensitive marker cookie: it carries no authorization
	// value and is never validated server-side. It exists purely so that
	// Edge middleware can tell "maybe still logged in, let the client resolve
	// it" apart from "definitely logged out" without being able to read the
	// HttpOnly refresh_token (whose Path is restricted to the refresh endpoint).
	// It must always be set/cleared alongside refresh_token with the exact same
	// MaxAge.
	c.SetCookie(h.authCookie("has_session", "1", hasSessionPath, sessionMaxAge))
}

func (h *Handler) clearAuthCookies(c echo.Context) {
	c.SetCookie(h.authCookie("access_token", "", accessTokenPath, -1))
	c.SetCookie(h.authCookie("refresh_token", "", refreshTokenPath, -1))
	c.SetCookie(h.authCookie("has_session", "", hasSessionPath, -1))
}
