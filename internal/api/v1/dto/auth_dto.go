package dto

// LoginRequest represents the login form data
type LoginRequest struct {
	// Email or Username
	EmailUsername string `form:"email-username" binding:"required" example:"admin"`
	// Password
	Password string `form:"password" binding:"required" example:"password"`
	// Captcha value entered by user
	Captcha string `form:"captcha"`
	// Captcha ID
	CaptchaID string `form:"captcha_id"`
	// Remember Me
	RememberMe bool `form:"remember-me"`
}

// ForgotPasswordRequest represents the forgot password form data
type ForgotPasswordRequest struct {
	// Email address
	Email string `form:"email" binding:"required" example:"user@example.com"`
	// Captcha value
	Captcha string `form:"captcha" binding:"required"`
}

// ResetPasswordRequest represents the reset password form data
type ResetPasswordRequest struct {
	// Email address
	Email string `form:"email" binding:"required" example:"user@example.com"`
	// Token Data
	TokenData string `form:"token_data" binding:"required"`
	// New Password
	Password string `form:"password" binding:"required" example:"newpassword123"`
	// Confirm Password
	ConfirmPassword string `form:"confirm-password" binding:"required" example:"newpassword123"`
}

// CaptchaRequest represents the captcha verification request
type CaptchaRequest struct {
	Captcha   string `form:"captcha" binding:"required"`
	CaptchaID string `form:"captcha_id" binding:"required"`
}
