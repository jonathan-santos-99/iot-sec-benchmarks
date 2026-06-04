package auth

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"
	"strings"
)

type Service struct {
	pwfile   string
	sessions map[string]string
}

func NewService(pwfile string) *Service {
	return &Service{pwfile, make(map[string]string)}
}

func (s *Service) IsLogged(token string) (string, bool) {
	username, ok := s.sessions[token]
	return username, ok
}

func (s *Service) CreateSessionCookie(username, password string) (string, bool) {
	userdata, ok := readUsers(s.pwfile)
	if !ok {
		return "", false
	}

	actually_user_password, found := userdata[username]
	if !found {
		log.Printf("User %s not found\n", username)
		return "", false
	}

	digest := sha256.Sum256([]byte(password))

	if actually_user_password != hex.EncodeToString(digest[:]) {
		log.Printf("Invalid password for user %s\n", username)
		return "", false
	}

	cookie, err := generateNewCookie(username)
	if err != nil {
		log.Printf("Could not generate cookie for %s\n", username)
		return "", false
	}

	s.sessions[cookie] = username
	return cookie, true
}

func generateNewCookie(username string) (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return username + "_" + hex.EncodeToString(b), nil
}

func readUsers(pwfile string) (map[string]string, bool) {
	file, err := os.Open(pwfile)
	if err != nil {
		log.Printf("Could not read pwfile %s: %s\n", pwfile, err)
		return nil, false
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	userdata := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ";")
		if len(parts) == 2 {
			username := parts[0]
			password := parts[1]
			userdata[username] = password
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error during scan: %s", err)
		return nil, false
	}

	return userdata, true
}
