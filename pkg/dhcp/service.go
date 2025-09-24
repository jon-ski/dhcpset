package dhcp

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/jon-ski/dhcpset/internal/logging"
)

// SetIPRequest represents a request to set an IP address for a device
type SetIPRequest struct {
	IP  net.IP
	MAC net.HardwareAddr
	XID uint32
}

// SetIPResult represents the result of setting an IP address
type SetIPResult struct {
	Success bool
	Error   error
	Message string // User-friendly message
}

// ProgressUpdate represents a progress update during IP assignment
type ProgressUpdate struct {
	Step          string
	Message       string
	Progress      int // 0-100
	TimeRemaining time.Duration
}

// SetIPLogMsg represents a log message during IP setting process
type SetIPLogMsg struct {
	Timestamp time.Time
	Message   string
}

// NewSetIPLogMsg creates a new log message with current timestamp
func NewSetIPLogMsg(msg string) SetIPLogMsg {
	return SetIPLogMsg{
		Timestamp: time.Now(),
		Message:   msg,
	}
}

// String returns a formatted string representation of the log message
func (m SetIPLogMsg) String() string {
	return fmt.Sprintf(
		"%s | %s",
		m.Timestamp.Local().Format("15:04:05"),
		m.Message,
	)
}

// Service provides a high-level interface for DHCP operations
type Service struct {
	server           *Server
	logger           func(string)         // Logging callback for UI updates
	progressCallback func(ProgressUpdate) // Progress callback for UI updates
	timeout          time.Duration        // Timeout for operations
}

// NewService creates a new DHCP service
func NewService(server *Server) *Service {
	return &Service{
		server: server,
		logger: func(msg string) {
			logging.Debug(msg)
		},
		timeout: 15 * time.Second, // Default 15 second timeout
	}
}

// SetLogger sets a custom logging function for the service
func (s *Service) SetLogger(logger func(string)) {
	s.logger = logger
}

// SetProgressCallback sets a callback for progress updates
func (s *Service) SetProgressCallback(callback func(ProgressUpdate)) {
	s.progressCallback = callback
}

// SetTimeout sets the timeout for DHCP operations
func (s *Service) SetTimeout(timeout time.Duration) {
	s.timeout = timeout
}

// SetIPAddress performs the complete DHCP handshake to set an IP address
func (s *Service) SetIPAddress(req SetIPRequest) SetIPResult {
	const maxAttempts = 3
	const baseBackoff = 500 * time.Millisecond

	macStr := req.MAC.String()
	ipStr := req.IP.String()

	logging.Info("dhcp_operation_started", "mac", macStr, "ip", ipStr, "xid", req.XID, "timeout_seconds", s.timeout.Seconds(), "max_attempts", maxAttempts)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * baseBackoff
			logging.Info("dhcp_retry_attempt", "attempt", attempt+1, "backoff_ms", backoff.Milliseconds(), "mac", macStr, "ip", ipStr, "xid", req.XID)
			s.updateProgress("retrying", fmt.Sprintf("🔄 Retrying IP assignment (attempt %d/%d)...", attempt+1, maxAttempts), 10, s.timeout)
			time.Sleep(backoff)
		} else {
			s.updateProgress("initializing", "🚀 Starting IP address assignment...", 0, s.timeout)
		}

		result := s.attemptSetIP(req, attempt+1)
		if result.Success {
			return result
		}

		// Log the attempt failure
		logging.Warn("dhcp_attempt_failed", "attempt", attempt+1, "error", result.Error, "mac", macStr, "ip", ipStr, "xid", req.XID)
	}

	// All attempts failed
	logging.Error("dhcp_operation_failed_all_attempts", "mac", macStr, "ip", ipStr, "xid", req.XID, "attempts", maxAttempts)
	return SetIPResult{
		Success: false,
		Error:   fmt.Errorf("failed after %d attempts", maxAttempts),
		Message: fmt.Sprintf("❌ Failed to assign IP after %d attempts. This usually means:\n• Device is not responding to DHCP\n• Network connection issues\n• Device may not support DHCP\n\nPlease check your device connection and try again", maxAttempts),
	}
}

// attemptSetIP performs a single attempt at the DHCP handshake
func (s *Service) attemptSetIP(req SetIPRequest, attempt int) SetIPResult {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	macStr := req.MAC.String()
	ipStr := req.IP.String()

	// Step 1: Send DHCP Offer
	s.updateProgress("contacting", "📡 Contacting your device...", 20, s.timeout)
	logging.Info("sending_dhcp_offer", "mac", macStr, "ip", ipStr, "xid", req.XID, "attempt", attempt)

	start := time.Now()
	if err := s.server.Offer(req.MAC, req.IP, req.XID); err != nil {
		logging.Error("failed_to_send_offer", "error", err, "mac", macStr, "ip", ipStr, "xid", req.XID, "attempt", attempt)
		return SetIPResult{
			Success: false,
			Error:   fmt.Errorf("failed to send offer: %w", err),
			Message: "❌ Couldn't contact your device. Please check:\n• Device is powered on and connected\n• Network cable is properly connected\n• Try again in a moment",
		}
	}
	logging.Info("dhcp_offer_sent", "mac", macStr, "ip", ipStr, "xid", req.XID, "duration_ms", time.Since(start).Milliseconds(), "attempt", attempt)

	// Step 2: Wait for DHCP Request with timeout
	s.updateProgress("waiting", "⏳ Waiting for device to respond...", 40, s.timeout)
	logging.Info("waiting_for_dhcp_request", "mac", macStr, "ip", ipStr, "xid", req.XID, "attempt", attempt)

	requestChan := make(chan error, 1)
	go func() {
		requestChan <- s.server.WaitRequest(req.MAC, req.IP, req.XID)
	}()

	select {
	case err := <-requestChan:
		if err != nil {
			logging.Error("failed_to_receive_request", "error", err, "mac", macStr, "ip", ipStr, "xid", req.XID, "attempt", attempt)
			return SetIPResult{
				Success: false,
				Error:   fmt.Errorf("failed to receive request: %w", err),
				Message: "❌ Device didn't respond in time. This might mean:\n• The device is busy or slow to respond\n• Network interference\n• Device doesn't support DHCP\n\nTry again or check device connection",
			}
		}
		logging.Info("dhcp_request_received", "mac", macStr, "ip", ipStr, "xid", req.XID, "attempt", attempt)
	case <-ctx.Done():
		logging.Error("timeout_waiting_for_request", "mac", macStr, "ip", ipStr, "xid", req.XID, "timeout_seconds", s.timeout.Seconds(), "attempt", attempt)
		return SetIPResult{
			Success: false,
			Error:   fmt.Errorf("timeout waiting for device request"),
			Message: "⏰ Device didn't respond in time. This usually means:\n• Device is busy or slow to respond\n• Network connection issues\n• Device may not support DHCP\n\nTry again or check your device connection",
		}
	}

	// Step 3: Send DHCP ACK
	s.updateProgress("confirming", "✅ Confirming IP address assignment...", 80, s.timeout)
	logging.Info("sending_dhcp_ack", "mac", macStr, "ip", ipStr, "xid", req.XID, "attempt", attempt)

	if err := s.server.Ack(req.MAC, req.IP, req.XID); err != nil {
		logging.Error("failed_to_send_ack", "error", err, "mac", macStr, "ip", ipStr, "xid", req.XID, "attempt", attempt)
		return SetIPResult{
			Success: false,
			Error:   fmt.Errorf("failed to send ACK: %w", err),
			Message: "❌ Couldn't confirm the IP assignment. Please try again.",
		}
	}
	logging.Info("dhcp_ack_sent", "mac", macStr, "ip", ipStr, "xid", req.XID, "attempt", attempt)

	s.updateProgress("complete", "🎉 IP address assigned successfully!", 100, 0)
	logging.Info("dhcp_operation_completed", "mac", macStr, "ip", ipStr, "xid", req.XID, "total_duration_ms", time.Since(start).Milliseconds(), "attempt", attempt)
	return SetIPResult{
		Success: true,
		Error:   nil,
		Message: "✅ Success! Your device now has the IP address " + req.IP.String(),
	}
}

// log is a helper method that uses the configured logger
func (s *Service) log(msg string) {
	if s.logger != nil {
		s.logger(msg)
	}
}

// updateProgress is a helper method that sends progress updates
func (s *Service) updateProgress(step, message string, progress int, timeRemaining time.Duration) {
	if s.progressCallback != nil {
		s.progressCallback(ProgressUpdate{
			Step:          step,
			Message:       message,
			Progress:      progress,
			TimeRemaining: timeRemaining,
		})
	}
	s.log(message)
}
