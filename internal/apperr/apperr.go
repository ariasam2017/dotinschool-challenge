// Package apperr is the single error shape crossing the service/HTTP boundary.
// It carries an HTTP status, a stable machine code, and a user-facing (Persian)
// message. Handlers return these; a middleware renders them as
// {"code":..., "message":...} with the right status. Anything that is not an
// *apperr.Error is treated as an unexpected 500 and its details never reach the
// client.
package apperr

import "fmt"

// Error is a typed HTTP error.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message)
}

// New builds an *Error.
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// As extracts an *Error from err, or nil.
func As(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e
	}
	return nil
}

// Common constructors mirroring the messages the original server sent.

func InvalidInput(msg string) *Error { return New(400, "INVALID_INPUT", msg) }
func InvalidPhone() *Error {
	return New(400, "INVALID_PHONE", "شماره موبایل باید ۱۰ رقم و بعد از +۹۸ باشد.")
}
func Unauthorized() *Error {
	return New(401, "UNAUTHORIZED", "نیاز به ورود مدیر است.")
}
func Forbidden(msg string) *Error { return New(403, "FORBIDDEN", msg) }
func NotFound(msg string) *Error  { return New(404, "NOT_FOUND", msg) }
func TooManyRequests() *Error {
	return New(429, "TOO_MANY_REQUESTS", "تعداد درخواست‌ها بیش از حد مجاز است — چند دقیقه دیگر دوباره تلاش کنید.")
}
func Internal() *Error {
	return New(500, "INTERNAL_ERROR", "خطای داخلی سرور رخ داد.")
}
