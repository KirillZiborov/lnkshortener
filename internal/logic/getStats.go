package logic

import (
	"errors"
	"net"
	"strings"
)

// GetStats retrieves server statistics from storage.
//
// Returns:
// - A number of shortened URLs.
// - A number of unique users.
// - An error if the query fails.
func (s *ShortenerService) GetStats() (int, int, error) {
	urlsCount, err := s.Store.GetURLsCount()
	if err != nil {
		return 0, 0, err
	}
	usersCount, err := s.Store.GetUsersCount()
	if err != nil {
		return 0, 0, err
	}
	return urlsCount, usersCount, nil
}

// ErrNoTrustedSubnet is returned when there is no trusted subnet specified.
var ErrNoTrustedSubnet = errors.New("no trusted subnet specified")

// ErrIPNotInSubnet indicates that a client IP address does not belong to the trusted subnet.
var ErrIPNotInSubnet = errors.New("client IP is not in trusted subnet")

// ErrNoClientIP is returned when there is no client IP provided.
var ErrNoClientIP = errors.New("client IP is not provided")

// CheckTrustedSubnet checks if the client IP belongs to the trusted subnet from configuration.
// It returns a corresponding error otherwise.
func (s *ShortenerService) CheckTrustedSubnet(clientIP string) error {
	if s.Cfg.TrustedSubnet == "" {
		return ErrNoTrustedSubnet
	}

	if clientIP == "" {
		return ErrNoClientIP
	}

	_, cidrNet, err := net.ParseCIDR(s.Cfg.TrustedSubnet)
	if err != nil {
		return err
	}

	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil || !cidrNet.Contains(ip) {
		return ErrIPNotInSubnet
	}

	return nil
}
