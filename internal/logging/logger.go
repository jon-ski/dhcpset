package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// Global logger instance
	Logger *slog.Logger
)

// Init initializes the global logger with file output
func Init() error {
	file, err := os.OpenFile("dhcpset.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize timestamp format
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(time.Now().Format("2006-01-02 15:04:05.000")),
				}
			}
			// Customize source information to use relative paths
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					return slog.Attr{
						Key: "source",
						Value: slog.GroupValue(
							slog.String("file", getRelativePath(source.File)),
							slog.Int("line", source.Line),
							slog.String("function", getFunctionName(source.Function)),
						),
					}
				}
			}
			return a
		},
	}

	Logger = slog.New(slog.NewJSONHandler(file, opts))
	return nil
}

// InitConsole initializes the global logger for console output
func InitConsole() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize timestamp format
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(time.Now().Format("2006-01-02 15:04:05.000")),
				}
			}
			// Customize source information to use relative paths
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					return slog.Attr{
						Key: "source",
						Value: slog.GroupValue(
							slog.String("file", getRelativePath(source.File)),
							slog.Int("line", source.Line),
							slog.String("function", getFunctionName(source.Function)),
						),
					}
				}
			}
			return a
		},
	}

	Logger = slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

// Convenience functions that use the global logger

func Debug(msg string, args ...interface{}) {
	Logger.Debug(msg, args...)
}

func Info(msg string, args ...interface{}) {
	Logger.Info(msg, args...)
}

func Warn(msg string, args ...interface{}) {
	Logger.Warn(msg, args...)
}

func Error(msg string, args ...interface{}) {
	Logger.Error(msg, args...)
}

func Fatal(msg string, args ...interface{}) {
	Logger.Error(msg, args...)
	os.Exit(1)
}

// Contextual logging helpers

func WithMAC(mac string) *slog.Logger {
	return Logger.With("mac", mac)
}

func WithIP(ip string) *slog.Logger {
	return Logger.With("ip", ip)
}

func WithXID(xid uint32) *slog.Logger {
	return Logger.With("xid", xid)
}

func WithOperation(operation string) *slog.Logger {
	return Logger.With("operation", operation)
}

// LogPacket logs packet information with context
func LogPacket(direction string, packetType string, mac string, ip string, xid uint32, additional ...interface{}) {
	args := []interface{}{
		"direction", direction,
		"packet_type", packetType,
		"mac", mac,
		"ip", ip,
		"xid", xid,
	}

	// Add additional fields
	for i := 0; i < len(additional)-1; i += 2 {
		if key, ok := additional[i].(string); ok {
			args = append(args, key, additional[i+1])
		}
	}

	Logger.Info("packet_"+direction, args...)
}

// LogDHCPOperation logs DHCP-specific operations
func LogDHCPOperation(operation string, mac string, ip string, xid uint32, err error) {
	if err != nil {
		Logger.Error("dhcp_operation_failed",
			"operation", operation,
			"mac", mac,
			"ip", ip,
			"xid", xid,
			"error", err.Error())
	} else {
		Logger.Info("dhcp_operation_success",
			"operation", operation,
			"mac", mac,
			"ip", ip,
			"xid", xid)
	}
}

// LogNetworkOperation logs network-specific operations
func LogNetworkOperation(operation string, interfaceName string, serverAddr string, err error) {
	if err != nil {
		Logger.Error("network_operation_failed",
			"operation", operation,
			"interface", interfaceName,
			"server_addr", serverAddr,
			"error", err.Error())
	} else {
		Logger.Info("network_operation_success",
			"operation", operation,
			"interface", interfaceName,
			"server_addr", serverAddr)
	}
}

// LogUIEvent logs UI-specific events
func LogUIEvent(event string, component string, data map[string]interface{}) {
	args := []interface{}{
		"event", event,
		"component", component,
	}

	for k, v := range data {
		args = append(args, k, v)
	}

	Logger.Info("ui_event", args...)
}

// Helper functions

// getRelativePath converts an absolute file path to a relative path from the project root
func getRelativePath(absPath string) string {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		// If we can't get the working directory, just return the filename
		return filepath.Base(absPath)
	}

	// Try to make the path relative to the working directory
	relPath, err := filepath.Rel(wd, absPath)
	if err != nil {
		// If that fails, try to make it relative to the module root
		// Look for go.mod file to find the module root
		dir := wd
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				relPath, err = filepath.Rel(dir, absPath)
				if err == nil {
					return relPath
				}
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
		// If all else fails, return just the filename
		return filepath.Base(absPath)
	}

	return relPath
}

// getFunctionName extracts a clean function name from the full function path
func getFunctionName(fullFunc string) string {
	if fullFunc == "" {
		return "unknown"
	}

	// Remove package path and keep only the function name
	// e.g., "github.com/user/project/pkg.function" -> "pkg.function"
	lastSlash := strings.LastIndex(fullFunc, "/")
	if lastSlash >= 0 {
		fullFunc = fullFunc[lastSlash+1:]
	}

	// If it's a method, keep the receiver type
	// e.g., "(*Logger).method" -> "(*Logger).method"
	if strings.Contains(fullFunc, "(") {
		return fullFunc
	}

	// For regular functions, try to keep package.function format
	// e.g., "package.function" -> "package.function"
	if strings.Contains(fullFunc, ".") {
		return fullFunc
	}

	return fullFunc
}
