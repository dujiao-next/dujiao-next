package domainhttps

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// Worker asks the local domain provisioning service to configure one database record.
type Worker struct {
	SocketPath string
}

func (w Worker) Setup(ctx context.Context, domainID uint) (string, error) {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "unix", w.SocketPath)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err := conn.SetDeadline(time.Now().Add(53 * time.Second)); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintf(conn, "%d\n", domainID); err != nil {
		return "", err
	}
	result, err := bufio.NewReader(io.LimitReader(conn, 64)).ReadString('\n')
	return strings.TrimSpace(result), err
}
