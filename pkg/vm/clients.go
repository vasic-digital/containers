package vm

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// HONESTY (clauses 6.J/6.L inherited from Containers' parent):
//
// v0.1 of pkg/vm ships the FAKE-driven hermetic test suite plus the
// operator's end-to-end real matrix run that lands in Phase C of the
// Lava-side consumer rollout. The real SSH/QMP client impls below
// are STUBBED with explicit "not implemented in v0.1" errors so any
// caller who reaches them sees an HONEST signal — not a silent no-op,
// not a panic, not a stub that pretends to work. Real impls land in
// a follow-up cycle (dedicated brainstorm). This is anti-bluff
// posture applied to the client code itself: don't ship code that
// pretends to work.

// defaultSSHClient returns a production sshClient that uses
// golang.org/x/crypto/ssh. The fake injection seam in qemu_test.go
// substitutes this for hermetic tests.
func defaultSSHClient() sshClient { return &realSSHClient{user: "root"} }

type realSSHClient struct {
	user   string
	client *ssh.Client
}

// WaitForListener does a plain TCP probe of 127.0.0.1:<port> with the
// given timeout — NO SSH handshake, NO userauth. Used by
// QEMUVM.WaitForReady to decide when the SSH listener is up.
//
// I4 fix: the previous implementation collapsed listener-up + handshake
// + ssh.Password("") userauth into a single Dial call. That combined
// path required the guest to accept empty-password root authentication,
// which essentially no real Linux server permits — so WaitForReady
// would always time out in production even on a fully-booted VM.
// Splitting listener-up out into this method matches what the unit
// test claims to verify (the listener became reachable) and what
// production needs (poll until SSH is accepting connections).
func (r *realSSHClient) WaitForListener(ctx context.Context, port int, timeout time.Duration) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// Authenticate opens a TCP connection to 127.0.0.1:<port> and performs
// the full SSH handshake + userauth. Empty-password root authentication
// is honoured ONLY by passwordless-root cloud-init Linux images
// (Alpine + cloud-init permits this when explicitly configured).
//
// PRODUCTION DEPLOYMENT NOTE: a real matrix run against arbitrary
// guest images requires either (a) the image is built with passwordless
// root + cloud-init, OR (b) a follow-up cycle implements key-based auth
// here. v0.1 ships the empty-password path so the operator's
// passwordless-root image can authenticate; clauses 6.J/6.L flag this
// as an honest known limitation rather than a bluff.
func (r *realSSHClient) Authenticate(ctx context.Context, port int, timeout time.Duration) error {
	cfg := &ssh.ClientConfig{
		User:            r.user,
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	c, ch, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return err
	}
	r.client = ssh.NewClient(c, ch, reqs)
	return nil
}

func (r *realSSHClient) Upload(ctx context.Context, hostPath, vmPath string) error {
	return fmt.Errorf("realSSHClient.Upload: not implemented in v0.1; operator end-to-end test only")
}

func (r *realSSHClient) Run(ctx context.Context, script string, env map[string]string, timeout time.Duration) (string, string, int, error) {
	return "", "", -1, fmt.Errorf("realSSHClient.Run: not implemented in v0.1; operator end-to-end test only")
}

func (r *realSSHClient) Download(ctx context.Context, vmPath, hostPath string) error {
	return fmt.Errorf("realSSHClient.Download: not implemented in v0.1; operator end-to-end test only")
}

func (r *realSSHClient) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// defaultQMPClient returns a production qmpClient. The hermetic test
// suite uses fakeQMPClient instead.
func defaultQMPClient() qmpClient { return &realQMPClient{} }

type realQMPClient struct{}

func (realQMPClient) Dial(ctx context.Context, port int, timeout time.Duration) error {
	return fmt.Errorf("realQMPClient.Dial: not implemented in v0.1; operator end-to-end test only")
}
func (realQMPClient) SystemPowerdown(ctx context.Context) error {
	return fmt.Errorf("realQMPClient.SystemPowerdown: not implemented in v0.1")
}
func (realQMPClient) Close() error { return nil }
