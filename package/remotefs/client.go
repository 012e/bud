package remotefs

import (
	"context"
	"encoding/gob"
	"io/fs"
	"strings"

	"github.com/keegancsmith/rpc"
	"github.com/livebud/bud/package/socket"
	"github.com/livebud/bud/package/virtual"
)

func init() {
	gob.Register(&virtual.File{})
	gob.Register(&virtual.DirEntry{})
}

func Dial(ctx context.Context, addr string) (*Client, error) {
	conn, err := socket.Dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	return NewClient(rpc.NewClient(conn)), nil
}

func NewClient(rpc *rpc.Client) *Client {
	return &Client{rpc, context.Background()}
}

type Client struct {
	rpc *rpc.Client
	ctx context.Context
}

var _ fs.FS = (*Client)(nil)
var _ fs.ReadDirFS = (*Client)(nil)

func (c *Client) WithContext(ctx context.Context) *Client {
	return &Client{c.rpc, ctx}
}

func (c *Client) Open(name string) (fs.File, error) {
	file := new(virtual.File)
	if err := c.rpc.Call(c.ctx, "remotefs.Open", name, file); err != nil {
		if isNotExist(err) && isNotWire(err) {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		return nil, err
	}
	return virtual.Open(file), nil
}

func (c *Client) ReadDir(name string) (des []fs.DirEntry, err error) {
	vdes := new([]fs.DirEntry)
	err = c.rpc.Call(c.ctx, "remotefs.ReadDir", name, &vdes)
	if err != nil {
		if isNotExist(err) && isNotWire(err) {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
		}
		return nil, err
	}
	return *vdes, nil
}

func (c *Client) Close() error {
	return c.rpc.Close()
}

// isNotExist is needed because the error has been serialized and passed between
// processes so errors.Is(err, fs.ErrNotExist) no longer is true.
func isNotExist(err error) bool {
	return strings.HasSuffix(err.Error(), fs.ErrNotExist.Error()) ||
		strings.HasSuffix(err.Error(), "no such file or directory")
}

// Dependency injection errors often pose as fs.ErrNotExist, but we don't want
// to treat them as such.
func isNotWire(err error) bool {
	return !strings.Contains(err.Error(), "unable to wire")
}
