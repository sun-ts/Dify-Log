package daemon

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const stopPath = "/api/v1/admin/stop"

func StopURL(host string, port int) string {
	return LocalURL(host, port, stopPath)
}

func LocalURL(host string, port int, path string) string {
	controlHost := strings.TrimSpace(host)
	if controlHost == "" || controlHost == "0.0.0.0" || controlHost == "::" {
		controlHost = "127.0.0.1"
	}
	controlHost = strings.TrimPrefix(strings.TrimSuffix(controlHost, "]"), "[")
	u := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(controlHost, strconv.Itoa(port)),
		Path:   path,
	}
	return u.String()
}

func RequestStop(ctx context.Context, stopURL string, apiKey string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	return RequestStopWithClient(ctx, client, stopURL, apiKey)
}

func RequestStopWithClient(ctx context.Context, client *http.Client, stopURL string, apiKey string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, stopURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("stop request failed with HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func WaitHTTPReady(ctx context.Context, readyURL string, pid int) error {
	client := http.Client{Timeout: 300 * time.Millisecond}
	var lastErr error
	for {
		if !ProcessRunning(pid) {
			if lastErr != nil {
				return fmt.Errorf("background process exited before HTTP server was ready: %w", lastErr)
			}
			return fmt.Errorf("background process exited before HTTP server was ready")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, readyURL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("HTTP server was not ready before timeout: %w", lastErr)
			}
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
