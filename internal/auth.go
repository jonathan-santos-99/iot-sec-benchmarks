package auth

import "strings"

type Service struct {
	allowedUsername string
	allowedPassword string
}

func NewService() *Service {
	return &Service{
		allowedUsername: strings.TrimSpace("jonathan"),
		allowedPassword: "123456",
	}
}

func (s *Service) CreateSessionCookie(username, password string) (string, bool) {
	ok := strings.TrimSpace(username) == s.allowedUsername && password == s.allowedPassword

	if !ok {
		return "", false
	}

	return "cookie", true
}
