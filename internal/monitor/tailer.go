package monitor

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"time"
)

// StartTailer watches and continuously tails the log file, passing lines to the collector.
func StartTailer(ctx context.Context, logPath string, collector *Collector) {
	slog.Info("Starting log tailer", "path", logPath)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		file, err := os.Open(logPath)
		if err != nil {
			slog.Debug("Waiting for log file to appear", "path", logPath, "err", err)
			time.Sleep(1 * time.Second)
			continue
		}

		slog.Info("Successfully opened log file for tailing", "path", logPath)
		reader := bufio.NewReader(file)

		for {
			select {
			case <-ctx.Done():
				file.Close()
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// Check if file was truncated or rotated
					info, statErr := file.Stat()
					if statErr == nil {
						curPos, seekErr := file.Seek(0, io.SeekCurrent)
						if seekErr == nil && info.Size() < curPos {
							slog.Info("Log file truncated or rotated, reopening", "path", logPath)
							break
						}
					}
					time.Sleep(200 * time.Millisecond)
					continue
				}
				slog.Warn("Error reading log line", "err", err)
				break
			}

			if err := collector.ProcessLine(line); err != nil {
				slog.Debug("Skipping unparseable log line", "err", err)
			}
		}

		file.Close()
		time.Sleep(1 * time.Second)
	}
}
