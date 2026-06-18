package gitutil

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

type Options struct {
	Timeout    time.Duration
	Attempts   int
	RetryDelay time.Duration
}

// go-git 只能通过全局 protocol registry 注入 HTTP client。
// go-git 读取该 map 时不加锁，因此必须在整个 git 操作期间保持 override 稳定。
var protocolMu sync.Mutex

var removeAll = os.RemoveAll

func DefaultOptions() Options {
	return Options{
		Timeout:    60 * time.Second,
		Attempts:   3,
		RetryDelay: 500 * time.Millisecond,
	}
}

// Client 封装 git 操作的 retry/timeout 配置。
// marketplace 和 plugin 包共享同一个实现，避免重复。
// transport 在构造时创建一次，跨 retry 尝试复用连接池。
type Client struct {
	opts      Options
	transport *http.Transport
}

func NewClient(opts Options) *Client {
	opts = normalizeOptions(opts)
	return &Client{
		opts:      opts,
		transport: newGitTransport(opts.Timeout),
	}
}

// Run 执行 op，按配置做 transient retry。
func (c *Client) Run(op func() error) error {
	return c.retry(c.opts, op)
}

// RunOnce 执行单次 op，不做 retry。
// 用于不需要 retry 的路径（如测试或显式单次尝试场景）。
func (c *Client) RunOnce(op func() error) error {
	opts := c.opts
	opts.Attempts = 1
	return c.retry(opts, op)
}

// CloneWithCleanup 执行 cloneFn，retry 前自动清理 partial clone 目录。
// 如果 cleanup 失败，错误信息包含上一次 clone 的原始错误，避免 mask 真正的失败原因。
func (c *Client) CloneWithCleanup(path string, cloneFn func() error) error {
	attempt := 0
	var lastCloneErr error
	return c.Run(func() error {
		if attempt > 0 {
			if rmErr := removeAll(path); rmErr != nil {
				return fmt.Errorf("failed to clean partial clone before retry: %w (prior clone error: %v)", rmErr, lastCloneErr)
			}
		}
		attempt++
		err := cloneFn()
		if err != nil {
			lastCloneErr = err
		}
		return err
	})
}

func (c *Client) retry(opts Options, op func() error) error {
	var lastErr error
	for attempt := 1; attempt <= opts.Attempts; attempt++ {
		err := c.withHTTPTimeout(op)
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == opts.Attempts || !isTransientGitError(err) {
			break
		}
		if opts.RetryDelay > 0 {
			time.Sleep(opts.RetryDelay)
		}
	}

	if opts.Attempts > 1 && isTransientGitError(lastErr) {
		return fmt.Errorf("git operation failed after %d attempts: %w", opts.Attempts, lastErr)
	}
	return lastErr
}

// withHTTPTimeout 安装自定义 HTTP client，执行 op，然后恢复。
// 锁覆盖整个 op 是有意的：go-git 的 protocol registry 是全局状态。
func (c *Client) withHTTPTimeout(op func() error) error {
	if c.transport == nil {
		return op()
	}

	httpClient := githttp.NewClient(&http.Client{Transport: c.transport})

	protocolMu.Lock()
	oldHTTP, hadHTTP := client.Protocols["http"]
	oldHTTPS, hadHTTPS := client.Protocols["https"]
	client.InstallProtocol("http", httpClient)
	client.InstallProtocol("https", httpClient)

	defer func() {
		restoreProtocol("http", oldHTTP, hadHTTP)
		restoreProtocol("https", oldHTTPS, hadHTTPS)
		protocolMu.Unlock()
	}()

	return op()
}

// IsTransientError 判断 git/network 错误是否适合 retry。
func IsTransientError(err error) bool {
	return isTransientGitError(err)
}

// RunWithRetry 是便捷函数，等价于 NewClient(opts).Run(op)。
func RunWithRetry(opts Options, op func() error) error {
	return NewClient(opts).Run(op)
}

func normalizeOptions(opts Options) Options {
	defaults := DefaultOptions()
	if opts.Timeout == 0 {
		opts.Timeout = defaults.Timeout
	}
	if opts.Attempts <= 0 {
		opts.Attempts = defaults.Attempts
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = defaults.RetryDelay
	}
	return opts
}

// newGitTransport 构造一个使用 transport 级别超时的 HTTP transport。
// 不设置 Client.Timeout（那是整个请求包括 body 传输的总超时），
// 因为大型 git clone 的 pack 数据传输可能合法地超过 60s。
// 取而代之，使用 DialContext / TLSHandshake / ResponseHeader 超时，
// 这样只有在连接建立或首字节等待阶段卡住时才超时，
// 正在传输数据（即使慢）的 clone 不会被中断。
func newGitTransport(timeout time.Duration) *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          10,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// newGitHTTPClient 暴露给测试验证 transport 配置。
func newGitHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Transport: newGitTransport(timeout)}
}

func restoreProtocol(scheme string, previous gittransport.Transport, existed bool) {
	if !existed {
		delete(client.Protocols, scheme)
		return
	}
	client.Protocols[scheme] = previous
}

func isTransientGitError(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	msg := strings.ToLower(err.Error())
	transientPhrases := []string{
		"connection reset by peer",
		"connection refused",
		"connection aborted",
		"connection reset",
		"deadline exceeded",
		"i/o timeout",
		"net/http: tls handshake timeout",
		"server closed idle connection",
		"temporary failure",
		"temporarily unavailable",
		"timeout",
		"tls handshake timeout",
		"unexpected eof",
	}
	for _, phrase := range transientPhrases {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	return false
}
